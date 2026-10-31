package mcpserver

import (
	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/sanitize"
)

// jsonSchemaDialect is the JSON Schema draft the generated schemas use.
const jsonSchemaDialect = "https://json-schema.org/draft/2020-12/schema"

// JSONSchema returns a JSON Schema (draft 2020-12) for t, a method's In or
// Out type in iface. Named types become entries in $defs referenced with
// $ref, so recursive types are fine. Nullable fields are optional and may be
// null; struct fields are otherwise required, and unknown fields are not
// allowed, matching schema.Validate. Doc comments become descriptions,
// sanitised.
func JSONSchema(iface *idl.Interface, t *idl.Type) map[string]any {
	g := schemaGen{iface: iface, defs: map[string]any{}}
	if t == nil {
		t = &idl.Type{Kind: idl.Struct}
	}
	root := g.typ(t)
	root["$schema"] = jsonSchemaDialect
	if len(g.defs) > 0 {
		root["$defs"] = g.defs
	}
	return root
}

// schemaGen collects $defs while converting types.
type schemaGen struct {
	iface *idl.Interface
	defs  map[string]any
}

// typ converts one type expression.
func (g *schemaGen) typ(t *idl.Type) map[string]any {
	switch t.Kind {
	case idl.Bool:
		return map[string]any{"type": "boolean"}
	case idl.Int:
		return map[string]any{"type": "integer"}
	case idl.Float:
		return map[string]any{"type": "number"}
	case idl.String:
		return map[string]any{"type": "string"}
	case idl.Object:
		return map[string]any{"type": "object"}
	case idl.Any:
		return map[string]any{}
	case idl.Nullable:
		return map[string]any{"anyOf": []any{g.typ(t.Elem), map[string]any{"type": "null"}}}
	case idl.Array:
		return map[string]any{"type": "array", "items": g.typ(t.Elem)}
	case idl.Map:
		return map[string]any{"type": "object", "additionalProperties": g.typ(t.Elem)}
	case idl.Enum:
		return enumSchema(t)
	case idl.Struct:
		return g.structSchema(t)
	case idl.Named:
		return g.named(t.Name)
	}
	return map[string]any{}
}

// named returns a $ref to the named type, defining it on first use. A name
// the interface does not define accepts anything, as schema.Validate does.
func (g *schemaGen) named(name string) map[string]any {
	ref := map[string]any{"$ref": "#/$defs/" + name}
	if _, ok := g.defs[name]; ok {
		return ref
	}
	var def *idl.TypeDef
	if g.iface != nil {
		def = g.iface.Type(name)
	}
	if def == nil || def.Type == nil {
		return map[string]any{"description": "type " + sanitize.Line(name) + " is not defined by the interface"}
	}
	g.defs[name] = map[string]any{} // placeholder, so a recursive reference stops here
	s := g.typ(def.Type)
	if def.Doc != "" {
		s["description"] = sanitize.Text(def.Doc)
	}
	g.defs[name] = s
	return ref
}

// structSchema converts a struct: an object with its fields as properties.
func (g *schemaGen) structSchema(t *idl.Type) map[string]any {
	props := map[string]any{}
	required := []string{}
	for _, f := range t.Fields {
		p := g.typ(f.Type)
		if f.Doc != "" {
			p["description"] = sanitize.Text(f.Doc)
		}
		props[f.Name] = p
		if f.Type.Kind != idl.Nullable {
			required = append(required, f.Name)
		}
	}
	s := map[string]any{"type": "object", "properties": props, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// enumSchema converts an enum to a string schema listing its values.
func enumSchema(t *idl.Type) map[string]any {
	values := make([]any, len(t.Values))
	for i, v := range t.Values {
		values[i] = v.Name
	}
	return map[string]any{"type": "string", "enum": values}
}
