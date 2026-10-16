// Package cmdline is the small grammar vlx's command line and `vlx call`
// share:
//
//	SERVICE [METHOD] [NAME=VALUE ...]
//
// for example `resolve ResolveHostname name=github.com family=2`. Words are
// split like a shell does: single or double quotes keep spaces, and a
// backslash escapes the next character. Values are converted to JSON using
// the method's IDL, so `name=123` stays a string for a string parameter and
// becomes a number for an int one.
package cmdline

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/rossedman/sdvarlink/idl"
)

// Arg is one NAME=VALUE pair, with VALUE unquoted.
type Arg struct {
	Name  string
	Value string
}

// Line is a parsed command line.
type Line struct {
	// Service is the first word, or "".
	Service string
	// Method is the second word when it is not an argument, or "".
	Method string
	// Args are the NAME=VALUE pairs, in order.
	Args []Arg
	// Trailing is true when the line ends in whitespace, so the last word
	// is complete.
	Trailing bool
}

// ErrUnterminated is returned for a quote that is never closed.
var ErrUnterminated = errors.New("unterminated quote")

// Split splits a line into words the way a POSIX shell does for quotes and
// backslashes (no variables, globs or operators).
func Split(line string) ([]string, error) {
	var (
		words []string
		cur   strings.Builder
		in    bool // inside a word
		quote rune
	)
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case quote != 0:
			switch {
			case r == quote:
				quote = 0
			case r == '\\' && quote == '"' && i+1 < len(runes) && (runes[i+1] == '"' || runes[i+1] == '\\'):
				i++
				cur.WriteRune(runes[i])
			default:
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote = r
			in = true
		case r == '\\' && i+1 < len(runes):
			i++
			cur.WriteRune(runes[i])
			in = true
		case r == ' ' || r == '\t' || r == '\n':
			if in {
				words = append(words, cur.String())
				cur.Reset()
				in = false
			}
		default:
			cur.WriteRune(r)
			in = true
		}
	}
	if quote != 0 {
		return nil, ErrUnterminated
	}
	if in {
		words = append(words, cur.String())
	}
	return words, nil
}

// Parse splits and interprets a line. Words containing '=' are arguments;
// the first two words that are not are the service and the method.
func Parse(line string) (Line, error) {
	words, err := Split(line)
	if err != nil {
		return Line{}, err
	}
	l := Line{Trailing: strings.HasSuffix(line, " ") || strings.HasSuffix(line, "\t")}
	for _, w := range words {
		if name, value, ok := strings.Cut(w, "="); ok && name != "" && isName(name) {
			l.Args = append(l.Args, Arg{Name: name, Value: value})
			continue
		}
		switch {
		case len(l.Args) > 0:
			return Line{}, fmt.Errorf("%q is not NAME=VALUE", w)
		case l.Service == "":
			l.Service = w
		case l.Method == "":
			l.Method = w
		default:
			return Line{}, fmt.Errorf("unexpected %q: arguments look like NAME=VALUE", w)
		}
	}
	return l, nil
}

func isName(s string) bool {
	for i, r := range s {
		ok := r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || i > 0 && r >= '0' && r <= '9'
		if !ok {
			return false
		}
	}
	return s != ""
}

// Quote quotes a word for the line when it needs it.
func Quote(s string) string {
	if s == "" {
		return "''"
	}
	plain := true
	for _, r := range s {
		if r <= ' ' || strings.ContainsRune(`'"\$`+"`", r) {
			plain = false
			break
		}
	}
	if plain {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// Format writes a line back, quoting values that need it.
func Format(service, method string, args []Arg) string {
	parts := make([]string, 0, 2+len(args))
	if service != "" {
		parts = append(parts, Quote(service))
	}
	if method != "" {
		parts = append(parts, Quote(method))
	}
	for _, a := range args {
		parts = append(parts, a.Name+"="+Quote(a.Value))
	}
	return strings.Join(parts, " ")
}

// Resolve strips Nullable from t and follows named types in iface.
func Resolve(iface *idl.Interface, t *idl.Type) (base *idl.Type, optional bool) {
	if t.Kind == idl.Nullable {
		optional = true
		t = t.Elem
	}
	for depth := 0; t.Kind == idl.Named && depth < 16 && iface != nil; depth++ {
		td := iface.Type(t.Name)
		if td == nil {
			break
		}
		t = td.Type
	}
	return t, optional
}

// Value converts one argument value to JSON for a parameter of type t.
// Strings and enum values are taken literally; everything else must be a
// JSON value, such as 42, true, [1,2] or {"a":1}.
func Value(iface *idl.Interface, t *idl.Type, raw string) (json.RawMessage, error) {
	base, _ := Resolve(iface, t)
	switch base.Kind {
	case idl.String, idl.Enum:
		b, _ := json.Marshal(raw)
		return b, nil
	}
	if !json.Valid([]byte(raw)) {
		return nil, fmt.Errorf("%q is not a valid %s", raw, base)
	}
	return json.RawMessage(raw), nil
}

// Params converts arguments to a JSON object of parameters for in, a
// method's input type, in argument order. Unknown names and repeated names
// are errors; validating the result against the IDL is the caller's job.
func Params(iface *idl.Interface, in *idl.Type, args []Arg) (json.RawMessage, error) {
	fields := map[string]*idl.Field{}
	if in != nil {
		for _, f := range in.Fields {
			fields[f.Name] = f
		}
	}
	seen := map[string]bool{}
	var b strings.Builder
	b.WriteByte('{')
	for i, a := range args {
		f := fields[a.Name]
		if f == nil {
			return nil, Unknown(a.Name, in)
		}
		if seen[a.Name] {
			return nil, Twice(a.Name)
		}
		seen[a.Name] = true
		v, err := Value(iface, f.Type, a.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", a.Name, err)
		}
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(a.Name)
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return json.RawMessage(b.String()), nil
}

// Unknown is the error for an argument that names no parameter of in.
func Unknown(name string, in *idl.Type) error {
	return fmt.Errorf("unknown parameter %q%s", name, suggest(name, in))
}

// Twice is the error for a parameter given more than once.
func Twice(name string) error {
	return fmt.Errorf("parameter %q given twice", name)
}

func suggest(name string, in *idl.Type) string {
	if in == nil {
		return ""
	}
	var names []string
	for _, f := range in.Fields {
		if strings.EqualFold(f.Name, name) {
			return fmt.Sprintf(" (did you mean %q?)", f.Name)
		}
		names = append(names, f.Name)
	}
	if len(names) == 0 {
		return " (this method takes no parameters)"
	}
	return " (parameters: " + strings.Join(names, ", ") + ")"
}

// Args converts a JSON object of parameters back to arguments, in the
// order of in's fields. Strings and enum values become bare words; other
// values keep their compact JSON.
func Args(iface *idl.Interface, in *idl.Type, params json.RawMessage) ([]Arg, error) {
	var obj map[string]json.RawMessage
	if len(params) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(params, &obj); err != nil {
		return nil, err
	}
	var out []Arg
	if in != nil {
		for _, f := range in.Fields {
			raw, ok := obj[f.Name]
			if !ok || string(raw) == "null" {
				continue
			}
			base, _ := Resolve(iface, f.Type)
			if base.Kind == idl.String || base.Kind == idl.Enum {
				var s string
				if json.Unmarshal(raw, &s) == nil {
					out = append(out, Arg{Name: f.Name, Value: s})
					continue
				}
			}
			out = append(out, Arg{Name: f.Name, Value: compact(raw)})
		}
	}
	return out, nil
}

func compact(raw json.RawMessage) string {
	var b strings.Builder
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	var v any
	if err := dec.Decode(&v); err != nil {
		return string(raw)
	}
	out, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	b.Write(out)
	return b.String()
}
