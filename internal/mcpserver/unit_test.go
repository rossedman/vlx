package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/rossedman/sdvarlink/idl"
	"github.com/rossedman/sdvarlink/varlink"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/schema"
)

// TestCallError maps every class of error.
func TestCallError(t *testing.T) {
	s := New(Options{})
	iface := mustParse(t, hostileIDL)
	te := &ToolError{Kind: KindLimit, Message: "kept"}
	tests := []struct {
		name string
		err  error
		kind string
		doc  string
	}{
		{"tool error passes through", fmt.Errorf("wrapped: %w", te), KindLimit, ""},
		{"deadline", fmt.Errorf("x: %w", context.DeadlineExceeded), KindLimit, ""},
		{"frame", varlink.ErrFrameTooLarge, KindLimit, ""},
		{"errno", &varlink.SystemError{Errno: 2}, KindErrno, ""},
		{"permission", varlink.ErrPermissionDenied, KindAccess, ""},
		{"eacces", fmt.Errorf("dial: %w", syscall.EACCES), KindAccess, ""},
		{"declared error", &varlink.Error{Name: "com.example.Hostile.Broken"}, KindVarlink, "Something went wrong."},
		{"other interface's error", &varlink.Error{Name: "com.example.Other.Broken"}, KindVarlink, ""},
		{"unqualified error", &varlink.Error{Name: "Broken"}, KindVarlink, ""},
		{"anything else", errors.New("connection refused"), KindUnavailable, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := s.callError(tc.err, iface)
			if got.Kind != tc.kind {
				t.Fatalf("kind = %s, want %s", got.Kind, tc.kind)
			}
			if tc.doc != "" && got.Varlink.Doc != tc.doc {
				t.Errorf("doc = %q", got.Varlink.Doc)
			}
			if !json.Valid([]byte(got.Error())) {
				t.Errorf("Error() is not JSON: %s", got.Error())
			}
		})
	}
	if got := s.callError(&varlink.Error{Name: "a.B"}, nil); got.Varlink.Doc != "" {
		t.Errorf("nil iface doc = %q", got.Varlink.Doc)
	}
}

// TestToolErrorText checks the fallback when the error cannot be encoded.
func TestToolErrorText(t *testing.T) {
	e := &ToolError{Kind: KindVarlink, Message: "m", Varlink: &VarlinkError{Parameters: make(chan int)}}
	if got := e.Error(); got != "varlink: m" {
		t.Errorf("Error() = %q", got)
	}
}

// TestInvalidParams covers each shape of validation error.
func TestInvalidParams(t *testing.T) {
	if got := invalidParams("m", &schema.Error{Path: "a", Msg: "bad"}); len(got.Params) != 1 || got.Params[0].Path != "a" {
		t.Errorf("one = %+v", got)
	}
	if got := invalidParams("m", errors.New("plain")); got.Params[0].Message != "plain" {
		t.Errorf("plain = %+v", got)
	}
}

// TestCutLast splits qualified names.
func TestCutLast(t *testing.T) {
	if p, l, ok := cutLast("a.b.C"); p != "a.b" || l != "C" || !ok {
		t.Errorf("a.b.C = %q %q %v", p, l, ok)
	}
	if p, l, ok := cutLast("C"); p != "" || l != "C" || ok {
		t.Errorf("C = %q %q %v", p, l, ok)
	}
}

// TestCleanValue covers empty, invalid and deeply nested input.
func TestCleanValue(t *testing.T) {
	s := New(Options{})
	if got := s.cleanValue(nil); got != nil {
		t.Errorf("nil = %v", got)
	}
	if got := s.cleanValue(json.RawMessage("{} trailing\x1b")); got != "{} trailing" {
		t.Errorf("invalid = %q", got)
	}
	deep := strings.Repeat("[", maxNesting+5) + strings.Repeat("]", maxNesting+5)
	b, _ := json.Marshal(s.cleanValue(json.RawMessage(deep)))
	if !strings.Contains(string(b), "nested too deeply") {
		t.Errorf("deep = %s", b)
	}
}

// TestParseResourceURI covers good and bad URIs.
func TestParseResourceURI(t *testing.T) {
	tests := []struct {
		uri, svc, iface string
		ok              bool
	}{
		{"varlink://io.systemd.Resolve/io.systemd.Resolve", "io.systemd.Resolve", "io.systemd.Resolve", true},
		{ResourceURI("a b", "c.D"), "a b", "c.D", true},
		{"file:///etc/passwd", "", "", false},
		{"varlink://onlyservice", "", "", false},
		{"varlink:///iface", "", "", false},
		{"varlink://svc/", "", "", false},
		{"varlink://svc/a/b", "", "", false},
		{"varlink://%zz/iface", "", "", false},
		{"varlink://a%2Fb/iface", "", "", false},
	}
	for _, tc := range tests {
		svc, iface, ok := parseResourceURI(tc.uri)
		if svc != tc.svc || iface != tc.iface || ok != tc.ok {
			t.Errorf("%s = %q %q %v", tc.uri, svc, iface, ok)
		}
	}
}

// TestTakesFiles finds fd fields at any depth.
func TestTakesFiles(t *testing.T) {
	iface := mustParse(t, hostileIDL)
	for name, want := range map[string]bool{"GetFile": true, "GetByName": false, "GetSelf": false, "Get": false} {
		if got := takesFiles(iface, iface.Method(name).In); got != want {
			t.Errorf("%s = %v, want %v", name, got, want)
		}
	}
	nested := &idl.Type{Kind: idl.Struct, Fields: []*idl.Field{{Name: "o", Type: &idl.Type{Kind: idl.Nullable,
		Elem: &idl.Type{Kind: idl.Struct, Fields: []*idl.Field{{Name: "stdinFileDescriptor", Type: &idl.Type{Kind: idl.Array, Elem: &idl.Type{Kind: idl.Int}}}}}}}}}
	if !takesFiles(iface, nested) {
		t.Error("nested fd not found")
	}
	if takesFiles(iface, &idl.Type{Kind: idl.Named, Name: "Undefined"}) || takesFiles(iface, nil) {
		t.Error("undefined or nil type takes files")
	}
}

// TestFindInterface covers ambiguous short names.
func TestFindInterface(t *testing.T) {
	svc := &client.Service{Interfaces: []*client.Interface{{Name: "a.X"}, {Name: "b.X"}, {Name: "c.Y"}}}
	_, err := findInterface(svc, "x")
	var te *ToolError
	if !errors.As(err, &te) || len(te.Candidates) != 2 || !strings.Contains(te.Message, "ambiguous") {
		t.Errorf("ambiguous = %v", err)
	}
	if got, err := findInterface(svc, "y"); err != nil || got.Name != "c.Y" {
		t.Errorf("y = %v, %v", got, err)
	}
	if got, err := findInterface(svc, "a.X"); err != nil || got.Name != "a.X" {
		t.Errorf("a.X = %v, %v", got, err)
	}
}

// TestStreamLimits clamps the stream argument.
func TestStreamLimits(t *testing.T) {
	s := New(Options{MCP: config.MCP{MaxStreamReplies: 5, MaxStreamSeconds: 3}})
	tests := []struct {
		in       *StreamInput
		rep, sec int
	}{
		{nil, 5, 3},
		{&StreamInput{}, 5, 3},
		{&StreamInput{MaxReplies: 50, MaxSeconds: 30}, 50, 30},
		{&StreamInput{MaxReplies: 5000, MaxSeconds: 5000}, config.MaxMCPStreamReplies, config.MaxMCPStreamSeconds},
		{&StreamInput{MaxReplies: -1, MaxSeconds: -1}, 5, 3},
	}
	for _, tc := range tests {
		if r, sec := s.streamLimits(tc.in); r != tc.rep || sec != tc.sec {
			t.Errorf("%+v = %d %d, want %d %d", tc.in, r, sec, tc.rep, tc.sec)
		}
	}
}

// TestDecodeCallInput rejects malformed arguments.
func TestDecodeCallInput(t *testing.T) {
	for _, args := range []string{`{"service":"a","method":"b","extra":1}`, `{"service":"a"}`, `{"service":"a","method":"b","stream":{"max_replies":1.5}}`, `[]`} {
		if _, err := decodeCallInput(json.RawMessage(args)); err == nil {
			t.Errorf("%s accepted", args)
		}
	}
	in, err := decodeCallInput(json.RawMessage(`{"service":"a","method":"b","params":{"id":18446744073709551615}}`))
	if err != nil || string(in.Params) != `{"id":18446744073709551615}` {
		t.Errorf("params = %s, %v", in.Params, err)
	}
}

// TestJSONSize handles values that cannot be encoded.
func TestJSONSize(t *testing.T) {
	if jsonSize(make(chan int)) != 0 || jsonSize("ab") != 4 {
		t.Error("jsonSize")
	}
}

// TestCleanNumber keeps representable numbers exact.
func TestCleanNumber(t *testing.T) {
	for in, want := range map[string]any{
		"1": json.Number("1"), "18446744073709551615": json.Number("18446744073709551615"),
		"-9223372036854775808": json.Number("-9223372036854775808"), "1.5e3": json.Number("1.5e3"), "1e400": "1e400",
	} {
		if got := cleanNumber(json.Number(in)); got != want {
			t.Errorf("%s = %#v, want %#v", in, got, want)
		}
	}
}

// TestStreamTimedOut covers the socket deadline firing before the
// context's own timer, which macOS runners hit.
func TestStreamTimedOut(t *testing.T) {
	now := time.Now()
	live := context.Background()
	gone, cancel := context.WithCancel(context.Background())
	cancel()
	tests := []struct {
		name        string
		ctx, runCtx context.Context
		err         error
		deadline    time.Time
		want        bool
	}{
		{"socket deadline before the context timer", live, live, context.DeadlineExceeded, now.Add(-time.Millisecond), true},
		{"socket deadline exactly at the deadline", live, live, context.DeadlineExceeded, now, true},
		{"context done", live, gone, context.Canceled, now.Add(time.Hour), true},
		{"timeout well before the deadline", live, live, context.DeadlineExceeded, now.Add(time.Second), false},
		{"other error after the deadline", live, live, errors.New("boom"), now.Add(-time.Second), false},
		{"client went away", gone, gone, context.Canceled, now.Add(-time.Second), false},
	}
	for _, tc := range tests {
		if got := streamTimedOut(tc.ctx, tc.runCtx, tc.err, tc.deadline, now); got != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, got, tc.want)
		}
	}
}
