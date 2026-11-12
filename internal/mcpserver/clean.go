package mcpserver

import (
	"bytes"
	"encoding/json"
	"strconv"

	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/sanitize"
)

// maxNesting bounds how deep cleanValue walks a reply.
const maxNesting = 128

// cleanValue decodes JSON from a service and returns it ready for the
// model: every key and string sanitised, and the values under redacted keys
// replaced with render.Redacted. Numbers keep their exact text unless no
// float64 can hold them. Empty input
// is nil; input that does not parse comes back as one sanitised string.
func (s *Server) cleanValue(raw json.RawMessage) any {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil || dec.More() {
		return sanitize.Text(string(raw))
	}
	return s.clean(v, 0)
}

// clean sanitises v recursively; past maxNesting it gives up and returns a
// placeholder rather than walking further.
func (s *Server) clean(v any, depth int) any {
	if depth > maxNesting {
		return "«nested too deeply»"
	}
	switch t := v.(type) {
	case string:
		return sanitize.Text(t)
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = s.clean(e, depth+1)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			key := sanitize.Line(k)
			if e != nil && s.redactor.Match(k) {
				out[key] = render.Redacted
				continue
			}
			out[key] = s.clean(e, depth+1)
		}
		return out
	case json.Number:
		return cleanNumber(t)
	default:
		return v // bool, nil
	}
}

// cleanNumber keeps a number's exact text when it is an integer or a finite
// float, and turns anything else, such as 1e400, into a string, because
// many JSON decoders reject numbers they cannot represent.
func cleanNumber(n json.Number) any {
	if _, err := strconv.ParseInt(string(n), 10, 64); err == nil {
		return n
	}
	if _, err := strconv.ParseUint(string(n), 10, 64); err == nil {
		return n
	}
	if _, err := n.Float64(); err == nil {
		return n
	}
	return string(n)
}
