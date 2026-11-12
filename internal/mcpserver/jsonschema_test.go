package mcpserver

import (
	"encoding/json"
	"testing"

	"github.com/rossedman/sdvarlink/idl"
)

// schemaIDL exercises every idl.Kind, nested and recursive types, doc
// comments and an undefined type reference.
const schemaIDL = `interface com.example.Schema

# A tree node.
type Node (
	# The node's label.
	label: string,
	children: []Node,
	parent: ?Node
)

type Color (red, green)

type Everything (
	b: bool,
	i: int,
	f: float,
	s: string,
	o: object,
	e: (one, two),
	arr: []int,
	m: [string]float,
	set: [string](),
	n: ?string,
	named: Color,
	tree: Node,
	inline: (x: int, y: ?int)
)

method Get(all: Everything, tag: ?string) -> (node: Node)
method Empty() -> ()
`

// mustParse parses an IDL or fails the test.
func mustParse(t *testing.T, src string) *idl.Interface {
	t.Helper()
	iface, err := idl.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return iface
}

// roundTrip marshals v to JSON and back, so tests compare plain values.
func roundTrip(t *testing.T, v any) map[string]any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return out
}

// dig walks a decoded JSON value by keys.
func dig(t *testing.T, v any, keys ...string) any {
	t.Helper()
	for _, k := range keys {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("at %q: not an object: %v", k, v)
		}
		v = m[k]
	}
	return v
}

// TestJSONSchemaKinds checks the schema of every kind of field.
func TestJSONSchemaKinds(t *testing.T) {
	iface := mustParse(t, schemaIDL)
	s := roundTrip(t, JSONSchema(iface, iface.Method("Get").In))
	if s["$schema"] != jsonSchemaDialect || s["type"] != "object" || s["additionalProperties"] != false {
		t.Fatalf("root = %v", s)
	}
	if req := dig(t, s, "required").([]any); len(req) != 1 || req[0] != "all" {
		t.Errorf("required = %v, want [all] (tag is nullable)", req)
	}
	every := dig(t, s, "$defs", "Everything", "properties")
	tests := []struct {
		field string
		want  string
	}{
		{"b", `{"type":"boolean"}`},
		{"i", `{"type":"integer"}`},
		{"f", `{"type":"number"}`},
		{"s", `{"type":"string"}`},
		{"o", `{"type":"object"}`},
		{"e", `{"enum":["one","two"],"type":"string"}`},
		{"arr", `{"items":{"type":"integer"},"type":"array"}`},
		{"m", `{"additionalProperties":{"type":"number"},"type":"object"}`},
		{"set", `{"additionalProperties":{"additionalProperties":false,"properties":{},"type":"object"},"type":"object"}`},
		{"n", `{"anyOf":[{"type":"string"},{"type":"null"}]}`},
		{"named", `{"$ref":"#/$defs/Color"}`},
		{"tree", `{"$ref":"#/$defs/Node"}`},
		{"inline", `{"additionalProperties":false,"properties":{"x":{"type":"integer"},"y":{"anyOf":[{"type":"integer"},{"type":"null"}]}},"required":["x"],"type":"object"}`},
	}
	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			got, err := json.Marshal(dig(t, every, tc.field))
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got  %s\nwant %s", got, tc.want)
			}
		})
	}
	if got := dig(t, s, "$defs", "Color", "enum").([]any); len(got) != 2 {
		t.Errorf("Color enum = %v", got)
	}
}

// TestJSONSchemaRecursive checks that a self-referencing type is defined
// once and referenced, with its doc comments as descriptions.
func TestJSONSchemaRecursive(t *testing.T) {
	iface := mustParse(t, schemaIDL)
	s := roundTrip(t, JSONSchema(iface, iface.Method("Get").Out))
	node := dig(t, s, "$defs", "Node")
	if dig(t, node, "description") != "A tree node." {
		t.Errorf("Node description = %v", dig(t, node, "description"))
	}
	if got := dig(t, node, "properties", "label", "description"); got != "The node's label." {
		t.Errorf("label description = %v", got)
	}
	if got := dig(t, node, "properties", "children", "items", "$ref"); got != "#/$defs/Node" {
		t.Errorf("children items = %v", got)
	}
	parent := dig(t, node, "properties", "parent", "anyOf").([]any)
	if dig(t, parent[0], "$ref") != "#/$defs/Node" {
		t.Errorf("parent = %v", parent)
	}
}

// TestJSONSchemaEdges covers a missing type, an empty struct and an
// undefined name.
func TestJSONSchemaEdges(t *testing.T) {
	iface := mustParse(t, schemaIDL)
	if s := roundTrip(t, JSONSchema(iface, nil)); s["type"] != "object" || s["required"] != nil {
		t.Errorf("nil type = %v", s)
	}
	if s := roundTrip(t, JSONSchema(iface, iface.Method("Empty").In)); s["$defs"] != nil {
		t.Errorf("empty = %v", s)
	}
	undefined := &idl.Type{Kind: idl.Struct, Fields: []*idl.Field{{Name: "x", Type: &idl.Type{Kind: idl.Named, Name: "Missing"}}}}
	s := roundTrip(t, JSONSchema(nil, undefined))
	if got := dig(t, s, "properties", "x", "description"); got != "type Missing is not defined by the interface" {
		t.Errorf("undefined = %v", got)
	}
	if got := roundTrip(t, map[string]any{"v": (&schemaGen{}).typ(&idl.Type{Kind: idl.Any})}); len(got["v"].(map[string]any)) != 0 {
		t.Errorf("any = %v", got)
	}
	if got := (&schemaGen{}).typ(&idl.Type{Kind: idl.Kind(99)}); len(got) != 0 {
		t.Errorf("unknown kind = %v", got)
	}
}
