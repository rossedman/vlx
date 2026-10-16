package render

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/rossedman/vlx/internal/sanitize"
)

// Redacted is shown in place of a redacted value.
const Redacted = "«redacted»"

// DefaultRedact are the key patterns redacted unless configured otherwise.
var DefaultRedact = []string{"*password*", "*secret*", "*privatekey*", "hashedpassword", "privileged", "*token*"}

// Redactor decides which object keys hide their values. Patterns are
// case-insensitive globs.
type Redactor struct {
	patterns []string
}

// NewRedactor returns a redactor for the given patterns; nil or empty
// disables redaction.
func NewRedactor(patterns []string) *Redactor {
	r := &Redactor{}
	for _, p := range patterns {
		r.patterns = append(r.patterns, strings.ToLower(p))
	}
	return r
}

// Match reports whether values under key are redacted.
func (r *Redactor) Match(key string) bool {
	if r == nil {
		return false
	}
	k := strings.ToLower(key)
	for _, p := range r.patterns {
		if ok, _ := path.Match(p, k); ok {
			return true
		}
	}
	return false
}

// node is a JSON value with object key order preserved.
type node struct {
	kind  byte // '{', '[', 's', 'n', 'b', '0' (null)
	text  string
	keys  []string
	elems []*node
}

func parse(data []byte) (*node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := parseValue(dec, 0)
	if err != nil {
		return nil, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after JSON value")
	}
	return n, nil
}

const maxNesting = 512

func parseValue(dec *json.Decoder, depth int) (*node, error) {
	if depth > maxNesting {
		return nil, errors.New("JSON nested too deeply")
	}
	tok, err := dec.Token()
	if err != nil {
		return nil, err
	}
	switch t := tok.(type) {
	case json.Delim:
		switch t {
		case '{':
			n := &node{kind: '{'}
			for dec.More() {
				kt, err := dec.Token()
				if err != nil {
					return nil, err
				}
				key, _ := kt.(string)
				v, err := parseValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				n.keys = append(n.keys, key)
				n.elems = append(n.elems, v)
			}
			_, err := dec.Token() // }
			return n, err
		case '[':
			n := &node{kind: '['}
			for dec.More() {
				v, err := parseValue(dec, depth+1)
				if err != nil {
					return nil, err
				}
				n.elems = append(n.elems, v)
			}
			_, err := dec.Token() // ]
			return n, err
		}
		return nil, fmt.Errorf("unexpected %v", t)
	case string:
		return &node{kind: 's', text: t}, nil
	case json.Number:
		return &node{kind: 'n', text: t.String()}, nil
	case bool:
		return &node{kind: 'b', text: fmt.Sprint(t)}, nil
	case nil:
		return &node{kind: '0', text: "null"}, nil
	}
	return nil, fmt.Errorf("unexpected token %v", tok)
}

// JSONOptions controls JSON rendering.
type JSONOptions struct {
	// Color enables syntax highlighting.
	Color bool
	// Redact hides values under matching keys; nil shows everything.
	Redact *Redactor
	// Indent is the indentation unit; two spaces when empty.
	Indent string
}

// JSON pretty-prints data, keeping object key order, sanitising every
// string for the terminal and redacting matching keys. Invalid JSON is
// returned sanitised but otherwise unchanged.
func JSON(data []byte, opts JSONOptions) string {
	n, err := parse(data)
	if err != nil {
		return sanitize.Text(string(data))
	}
	if opts.Indent == "" {
		opts.Indent = "  "
	}
	var b strings.Builder
	w := jsonWriter{b: &b, opts: opts}
	w.value(n, 0)
	return b.String()
}

type jsonWriter struct {
	b    *strings.Builder
	opts JSONOptions
}

func (w jsonWriter) style(s lipglossStyle, text string) {
	if w.opts.Color {
		w.b.WriteString(s.Render(text))
	} else {
		w.b.WriteString(text)
	}
}

func (w jsonWriter) value(n *node, depth int) {
	switch n.kind {
	case '{':
		if len(n.keys) == 0 {
			w.style(PunctStyle, "{}")
			return
		}
		w.style(PunctStyle, "{")
		w.b.WriteByte('\n')
		for i, k := range n.keys {
			w.b.WriteString(strings.Repeat(w.opts.Indent, depth+1))
			w.style(KeyStyle, quote(k))
			w.style(PunctStyle, ":")
			w.b.WriteByte(' ')
			if w.opts.Redact.Match(k) && n.elems[i].kind != '0' {
				w.style(RedactStyle, Redacted)
			} else {
				w.value(n.elems[i], depth+1)
			}
			if i < len(n.keys)-1 {
				w.style(PunctStyle, ",")
			}
			w.b.WriteByte('\n')
		}
		w.b.WriteString(strings.Repeat(w.opts.Indent, depth))
		w.style(PunctStyle, "}")
	case '[':
		if len(n.elems) == 0 {
			w.style(PunctStyle, "[]")
			return
		}
		w.style(PunctStyle, "[")
		w.b.WriteByte('\n')
		for i, e := range n.elems {
			w.b.WriteString(strings.Repeat(w.opts.Indent, depth+1))
			w.value(e, depth+1)
			if i < len(n.elems)-1 {
				w.style(PunctStyle, ",")
			}
			w.b.WriteByte('\n')
		}
		w.b.WriteString(strings.Repeat(w.opts.Indent, depth))
		w.style(PunctStyle, "]")
	case 's':
		w.style(StringStyle, quote(n.text))
	case 'n':
		w.style(NumberStyle, n.text)
	case 'b':
		w.style(BoolStyle, n.text)
	default:
		w.style(NullStyle, "null")
	}
}

// quote JSON-quotes s; control characters come out escaped, so the result
// is safe for the terminal.
func quote(s string) string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(s)
	return sanitize.Line(strings.TrimSuffix(b.String(), "\n"))
}
