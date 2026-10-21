// Package schema turns Varlink IDL types into JSON parameter skeletons and
// validates JSON parameters against them before anything is sent.
package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/rossedman/sdvarlink/idl"
)

// maxDepth bounds recursion through named types, which may refer to
// themselves (directly or through a cycle).
const maxDepth = 16

// Skeleton returns an indented JSON object for t (a method's In or Out
// type) with every required field set to a typed placeholder. Optional
// (nullable) fields are left out and returned, as "name: type" lines, in
// optional.
func Skeleton(iface *idl.Interface, t *idl.Type) (text string, optional []string) {
	if t == nil || t.Kind != idl.Struct || len(t.Fields) == 0 {
		return "{}", nil
	}
	var b bytes.Buffer
	sk := skeleton{iface: iface}
	b.WriteString("{\n")
	first := true
	for _, f := range t.Fields {
		if f.Type.Kind == idl.Nullable {
			optional = append(optional, f.Name+": "+f.Type.Elem.String())
			continue
		}
		if !first {
			b.WriteString(",\n")
		}
		first = false
		b.WriteString("  ")
		b.WriteString(strconv.Quote(f.Name))
		b.WriteString(": ")
		sk.value(&b, f.Type, 1, 0)
	}
	if first {
		return "{}", optional
	}
	b.WriteString("\n}")
	return b.String(), optional
}

type skeleton struct {
	iface *idl.Interface
}

// value writes a placeholder for t at the given indent: a zero value,
// the first enum value, or required struct fields only; past maxDepth it
// writes null.
func (s skeleton) value(b *bytes.Buffer, t *idl.Type, indent, depth int) {
	if depth > maxDepth {
		b.WriteString("null")
		return
	}
	switch t.Kind {
	case idl.Bool:
		b.WriteString("false")
	case idl.Int:
		b.WriteString("0")
	case idl.Float:
		b.WriteString("0.0")
	case idl.String:
		b.WriteString(`""`)
	case idl.Object, idl.Map:
		b.WriteString("{}")
	case idl.Any, idl.Nullable:
		b.WriteString("null")
	case idl.Array:
		b.WriteString("[]")
	case idl.Enum:
		if len(t.Values) > 0 {
			b.WriteString(strconv.Quote(t.Values[0].Name))
		} else {
			b.WriteString(`""`)
		}
	case idl.Named:
		if def := s.lookup(t.Name); def != nil {
			s.value(b, def.Type, indent, depth+1)
		} else {
			b.WriteString("null")
		}
	case idl.Struct:
		var required []*idl.Field
		for _, f := range t.Fields {
			if f.Type.Kind != idl.Nullable {
				required = append(required, f)
			}
		}
		if len(required) == 0 {
			b.WriteString("{}")
			return
		}
		pad := strings.Repeat("  ", indent+1)
		b.WriteString("{\n")
		for i, f := range required {
			b.WriteString(pad)
			b.WriteString(strconv.Quote(f.Name))
			b.WriteString(": ")
			s.value(b, f.Type, indent+1, depth+1)
			if i < len(required)-1 {
				b.WriteByte(',')
			}
			b.WriteByte('\n')
		}
		b.WriteString(strings.Repeat("  ", indent))
		b.WriteString("}")
	default:
		b.WriteString("null")
	}
}

// lookup finds the named type in the interface, or returns nil.
func (s skeleton) lookup(name string) *idl.TypeDef {
	if s.iface == nil {
		return nil
	}
	return s.iface.Type(name)
}

// Error is a validation failure at a JSON path such as ".addresses[2].family".
type Error struct {
	Path string
	Msg  string
}

// Error prefixes the message with the path when there is one.
func (e *Error) Error() string {
	if e.Path == "" {
		return e.Msg
	}
	return e.Path + ": " + e.Msg
}

// Errors is every validation failure found, in document order.
type Errors []*Error

// Error shows the first failure and how many more there are.
func (es Errors) Error() string {
	switch len(es) {
	case 0:
		return "no errors"
	case 1:
		return es[0].Error()
	}
	return fmt.Sprintf("%s (and %d more)", es[0].Error(), len(es)-1)
}

// Validate checks that params is a JSON object matching t, a method's In
// type. It returns nil, a *Error for malformed JSON, or Errors.
func Validate(iface *idl.Interface, t *idl.Type, params []byte) error {
	trimmed := bytes.TrimSpace(params)
	if len(trimmed) == 0 {
		trimmed = []byte("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(trimmed))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return &Error{Msg: "invalid JSON: " + jsonErrorMessage(err, trimmed)}
	}
	if dec.More() {
		return &Error{Msg: "invalid JSON: unexpected data after the object"}
	}
	if _, ok := v.(map[string]any); !ok {
		return &Error{Msg: "parameters must be a JSON object, got " + kindOf(v)}
	}
	if t == nil {
		t = &idl.Type{Kind: idl.Struct}
	}
	vl := validator{iface: iface}
	vl.check("", t, v, 0)
	if len(vl.errs) > 0 {
		return vl.errs
	}
	return nil
}

type validator struct {
	iface *idl.Interface
	errs  Errors
}

// fail records a failure at path.
func (vl *validator) fail(path, format string, args ...any) {
	vl.errs = append(vl.errs, &Error{Path: path, Msg: fmt.Sprintf(format, args...)})
}

// check validates v against t at path, recording every mismatch, missing
// required field and unknown field. Unknown named types are left to the
// service.
func (vl *validator) check(path string, t *idl.Type, v any, depth int) {
	if depth > maxDepth {
		return
	}
	switch t.Kind {
	case idl.Nullable:
		if v != nil {
			vl.check(path, t.Elem, v, depth+1)
		}
		return
	case idl.Any:
		return
	case idl.Named:
		def := vl.lookup(t.Name)
		if def == nil {
			return // unknown type: let the service decide
		}
		vl.check(path, def.Type, v, depth+1)
		return
	}
	if v == nil {
		vl.fail(path, "must not be null (type %s)", t)
		return
	}
	switch t.Kind {
	case idl.Bool:
		if _, ok := v.(bool); !ok {
			vl.fail(path, "expected bool, got %s", kindOf(v))
		}
	case idl.Int:
		n, ok := v.(json.Number)
		if !ok {
			vl.fail(path, "expected int, got %s", kindOf(v))
			return
		}
		if _, err := strconv.ParseInt(string(n), 10, 64); err != nil {
			if _, uerr := strconv.ParseUint(string(n), 10, 64); uerr != nil {
				vl.fail(path, "expected int, got %s", n)
			}
		}
	case idl.Float:
		if _, ok := v.(json.Number); !ok {
			vl.fail(path, "expected float, got %s", kindOf(v))
		}
	case idl.String:
		if _, ok := v.(string); !ok {
			vl.fail(path, "expected string, got %s", kindOf(v))
		}
	case idl.Object:
		if _, ok := v.(map[string]any); !ok {
			vl.fail(path, "expected object, got %s", kindOf(v))
		}
	case idl.Enum:
		s, ok := v.(string)
		if !ok {
			vl.fail(path, "expected one of %s, got %s", enumList(t), kindOf(v))
			return
		}
		for _, ev := range t.Values {
			if ev.Name == s {
				return
			}
		}
		vl.fail(path, "%q is not one of %s", s, enumList(t))
	case idl.Array:
		arr, ok := v.([]any)
		if !ok {
			vl.fail(path, "expected array, got %s", kindOf(v))
			return
		}
		for i, e := range arr {
			vl.check(fmt.Sprintf("%s[%d]", path, i), t.Elem, e, depth+1)
		}
	case idl.Map:
		m, ok := v.(map[string]any)
		if !ok {
			vl.fail(path, "expected object (map), got %s", kindOf(v))
			return
		}
		for _, k := range sortedKeys(m) {
			vl.check(fmt.Sprintf("%s[%q]", path, k), t.Elem, m[k], depth+1)
		}
	case idl.Struct:
		m, ok := v.(map[string]any)
		if !ok {
			vl.fail(path, "expected object, got %s", kindOf(v))
			return
		}
		known := make(map[string]bool, len(t.Fields))
		for _, f := range t.Fields {
			known[f.Name] = true
			fv, present := m[f.Name]
			if !present {
				if f.Type.Kind != idl.Nullable {
					vl.fail(path+"."+f.Name, "required field is missing (type %s)", f.Type)
				}
				continue
			}
			vl.check(path+"."+f.Name, f.Type, fv, depth+1)
		}
		for _, k := range sortedKeys(m) {
			if !known[k] {
				vl.fail(path+"."+k, "unknown field%s", suggest(k, t.Fields))
			}
		}
	}
}

// lookup finds the named type in the interface, or returns nil.
func (vl *validator) lookup(name string) *idl.TypeDef {
	if vl.iface == nil {
		return nil
	}
	return vl.iface.Type(name)
}

// kindOf names the JSON kind of a decoded value for error messages.
func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case json.Number:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return fmt.Sprintf("%T", v)
}

// enumList formats t's enum values as a quoted, bracketed list.
func enumList(t *idl.Type) string {
	names := make([]string, len(t.Values))
	for i, v := range t.Values {
		names[i] = strconv.Quote(v.Name)
	}
	return "[" + strings.Join(names, ", ") + "]"
}

// sortedKeys returns m's keys in order, so errors come out deterministic.
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// suggest returns ` (did you mean "x"?)` for a field name that differs from
// a known one only in case, or "".
func suggest(name string, fields []*idl.Field) string {
	for _, f := range fields {
		if strings.EqualFold(f.Name, name) {
			return fmt.Sprintf(" (did you mean %q?)", f.Name)
		}
	}
	return ""
}

// jsonErrorMessage adds the line and column to a JSON syntax error.
func jsonErrorMessage(err error, data []byte) string {
	var se *json.SyntaxError
	if ok := asSyntax(err, &se); ok {
		line, col := position(data, se.Offset)
		return fmt.Sprintf("%v at line %d, column %d", se, line, col)
	}
	return err.Error()
}

// asSyntax reports whether err is a *json.SyntaxError and stores it in
// target.
func asSyntax(err error, target **json.SyntaxError) bool {
	se, ok := err.(*json.SyntaxError) //nolint:errorlint // json returns it unwrapped
	if ok {
		*target = se
	}
	return ok
}

// position converts a decoder byte offset into a 1-based line and column.
func position(data []byte, offset int64) (line, col int) {
	line, col = 1, 1
	for i := int64(0); i < offset-1 && i < int64(len(data)); i++ {
		if data[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return line, col
}
