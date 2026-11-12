package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/schema"
)

// Reasons a call stopped reading replies before the service ended the
// stream, the stopped field of CallOutput.
const (
	// StoppedMaxReplies: stream.max_replies replies were read.
	StoppedMaxReplies = "max_replies"
	// StoppedMaxSeconds: stream.max_seconds passed.
	StoppedMaxSeconds = "max_seconds"
	// StoppedMaxResultBytes: the next reply would have made the result
	// larger than the server's result budget.
	StoppedMaxResultBytes = "max_result_bytes"
)

// CallInput is the input of call.
type CallInput struct {
	Service string          `json:"service"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	Stream  *StreamInput    `json:"stream,omitempty"`
}

// StreamInput bounds a streaming call.
type StreamInput struct {
	MaxReplies int `json:"max_replies,omitempty"`
	MaxSeconds int `json:"max_seconds,omitempty"`
}

// CallOutput is the result of call.
type CallOutput struct {
	Method string `json:"method"`
	Class  string `json:"class"`
	// Replies holds each reply's parameters, sanitised and redacted.
	Replies []any `json:"replies"`
	// Continues is true when the service had more replies to send.
	Continues bool `json:"continues"`
	// Stopped says which limit ended a stream early.
	Stopped   string `json:"stopped,omitempty"`
	ElapsedMs int64  `json:"elapsed_ms"`
	// Files counts file descriptors that came with the replies; vlx closes
	// them unread.
	Files int `json:"files,omitempty"`
	// Truncated is true when replies were dropped to fit the result budget.
	Truncated bool   `json:"truncated,omitempty"`
	Notice    string `json:"notice"`
}

// callInputSchema is the input schema of call, written by hand because
// params is free-form JSON that schema inference would get wrong.
func callInputSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"service", "method"},
		"properties": map[string]any{
			"service": map[string]any{"type": "string",
				"description": "a short name (resolve), a full name (io.systemd.Resolve) or a socket path"},
			"method": map[string]any{"type": "string",
				"description": "qualified (io.systemd.Resolve.ResolveHostname) or bare when only one interface has it"},
			"params": map[string]any{"type": "object",
				"description": "the method's parameters; see method_schema"},
			"stream": map[string]any{
				"type":        "object",
				"description": "read several replies from a method that streams; methods that require streaming always stream",
				"properties": map[string]any{
					"max_replies": map[string]any{"type": "integer", "minimum": 1, "maximum": 200,
						"description": "stop after this many replies (default 20)"},
					"max_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": 60,
						"description": "stop after this many seconds (default 10)"},
				},
				"additionalProperties": false,
			},
		},
		"additionalProperties": false,
	}
}

// callTool handles call. It is a raw tool handler rather than a typed one
// because the SDK's typed handlers decode arguments and results through
// float64, which would round 64-bit integers in params and replies; here
// both keep their exact digits.
func (s *Server) callTool(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	in, err := decodeCallInput(req.Params.Arguments)
	if err == nil {
		var out CallOutput
		if out, err = s.call(ctx, in); err == nil {
			b, err := json.Marshal(out)
			if err != nil {
				return nil, err
			}
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(b)}}, StructuredContent: json.RawMessage(b)}, nil
		}
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}, IsError: true}, nil
}

// decodeCallInput decodes call's arguments strictly, keeping params as
// raw JSON.
func decodeCallInput(args json.RawMessage) (CallInput, error) {
	var in CallInput
	dec := json.NewDecoder(bytes.NewReader(args))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return in, &ToolError{Kind: KindInvalidParams, Retry: true, Message: "arguments: " + err.Error(),
			Hint: "Pass service, method, and optionally params (an object) and stream."}
	}
	if in.Service == "" || in.Method == "" {
		return in, &ToolError{Kind: KindInvalidParams, Retry: true, Message: "service and method are required"}
	}
	return in, nil
}

// call resolves the method, refuses what the policy does not allow,
// validates the parameters and reads the replies within the stream and
// result limits.
func (s *Server) call(ctx context.Context, in CallInput) (CallOutput, error) {
	path, iface, m, err := s.findMethod(ctx, in.Service, in.Method)
	if err != nil {
		return CallOutput{}, err
	}
	p := iface.Parsed
	q := p.Name + "." + m.Name
	v := s.classifier.ClassifyCall(q, false)
	if r := s.refusal(p, m, q, v); r != "" {
		return CallOutput{}, &ToolError{Kind: KindRefused, Message: r, Class: v.Class.String(),
			Hint: "Use a READ method instead, or ask the person to run this themselves."}
	}
	params, err := callParams(p, m, in.Params)
	if err != nil {
		return CallOutput{}, invalidParams(q, err)
	}
	more, err := wantsMore(m, in.Stream)
	if err != nil {
		return CallOutput{}, err
	}
	maxReplies, maxSeconds := s.streamLimits(in.Stream)

	out := CallOutput{Method: q, Class: v.Class.String(), Replies: []any{}, Notice: Notice}
	start := s.now()
	runCtx := ctx
	var deadline time.Time
	if more {
		var cancel context.CancelFunc
		deadline = time.Now().Add(time.Duration(maxSeconds) * time.Second)
		runCtx, cancel = context.WithDeadline(ctx, deadline)
		defer cancel()
	}
	size := 0
	err = client.Call(runCtx, path, client.Request{Method: q, Params: params, More: more}, s.opts.Client, func(r client.Reply) error {
		out.Files += r.Files
		out.Continues = r.Continues
		val := s.cleanValue(r.Params)
		n := jsonSize(val)
		if size+n > s.opts.MCP.MaxResultBytes {
			if len(out.Replies) == 0 {
				return errTooLarge
			}
			out.Stopped, out.Truncated, out.Continues = StoppedMaxResultBytes, true, true
			return client.ErrStop
		}
		size += n
		out.Replies = append(out.Replies, val)
		if more && r.Continues && len(out.Replies) >= maxReplies {
			out.Stopped = StoppedMaxReplies
			return client.ErrStop
		}
		return nil
	})
	out.ElapsedMs = s.now().Sub(start).Milliseconds()
	timedOut := more && streamTimedOut(ctx, runCtx, err, deadline, time.Now())
	switch {
	case err == nil:
	case timedOut && len(out.Replies) > 0:
		// The stream outlived max_seconds: a normal end, not a failure.
		out.Stopped, out.Continues = StoppedMaxSeconds, true
	case timedOut:
		return CallOutput{}, &ToolError{Kind: KindLimit, Retry: true,
			Message: "no reply arrived within stream.max_seconds",
			Hint:    "Raise stream.max_seconds (at most 60) if the method is slow to start."}
	case errors.Is(err, errTooLarge):
		return CallOutput{}, &ToolError{Kind: KindLimit,
			Message: "the reply is larger than this server's result budget",
			Hint:    "Narrow the request, for example with a filter parameter, so the reply is smaller."}
	default:
		return CallOutput{}, s.callError(err, p)
	}
	return out, nil
}

// streamTimedOut reports whether a stream ended because max_seconds passed
// rather than because the client went away or the service failed. The
// connection's read deadline is set to the same instant as runCtx's, and
// the socket can time out a moment before the context's timer marks
// runCtx done, so a deadline error at or after the deadline counts too.
func streamTimedOut(ctx, runCtx context.Context, err error, deadline, now time.Time) bool {
	if ctx.Err() != nil {
		return false
	}
	return runCtx.Err() != nil || (errors.Is(err, context.DeadlineExceeded) && !now.Before(deadline))
}

// errTooLarge ends a call whose first reply does not fit the result budget.
var errTooLarge = errors.New("reply exceeds the result budget")

// callParams checks raw against m's input type and returns it compacted;
// a missing or null params is the empty object.
func callParams(iface *idl.Interface, m *idl.Method, raw json.RawMessage) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		raw = json.RawMessage("{}")
	}
	if err := schema.Validate(iface, m.In, raw); err != nil {
		return nil, err
	}
	return client.Compact(raw)
}

// wantsMore decides whether to ask for several replies: always for methods
// that require it, only on request for methods that support it, and never
// for single-reply methods, which reject a stream argument.
func wantsMore(m *idl.Method, stream *StreamInput) (bool, error) {
	switch m.More {
	case idl.MoreRequired:
		return true, nil
	case idl.MoreSupported:
		return stream != nil, nil
	}
	if stream != nil {
		return false, &ToolError{Kind: KindInvalidParams, Retry: true,
			Message: m.Name + " returns a single reply and does not stream",
			Hint:    "Call it again without stream."}
	}
	return false, nil
}

// streamLimits returns the reply and time limits for a call: the configured
// defaults, or the stream argument's values clamped to the hard caps.
func (s *Server) streamLimits(stream *StreamInput) (replies, seconds int) {
	replies, seconds = s.opts.MCP.MaxStreamReplies, s.opts.MCP.MaxStreamSeconds
	if stream == nil {
		return replies, seconds
	}
	if stream.MaxReplies > 0 {
		replies = min(stream.MaxReplies, config.MaxMCPStreamReplies)
	}
	if stream.MaxSeconds > 0 {
		seconds = min(stream.MaxSeconds, config.MaxMCPStreamSeconds)
	}
	return replies, seconds
}

// jsonSize is the length of v encoded as JSON.
func jsonSize(v any) int {
	b, err := json.Marshal(v)
	if err != nil {
		return 0
	}
	return len(b)
}
