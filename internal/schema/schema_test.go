package schema

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rossedman/sdvarlink/idl"
)

const testIDL = `
interface com.example.Test

type Family (inet, inet6)

type Address (
  family: Family,
  address: []int,
  label: ?string
)

type Node (
  name: string,
  children: []Node
)

method Lookup(
  name: string,
  ifindex: ?int,
  flags: int,
  ratio: float,
  family: Family,
  addresses: []Address,
  labels: [string]string,
  extra: object,
  anything: ?any,
  enabled: bool,
  root: Node,
  inline: (a: int, b: ?bool)
) -> ()

method Nothing() -> ()
`

func parse(t *testing.T) *idl.Interface {
	t.Helper()
	iface, err := idl.Parse(testIDL)
	if err != nil {
		t.Fatal(err)
	}
	return iface
}

func TestSkeleton(t *testing.T) {
	iface := parse(t)
	text, optional := Skeleton(iface, iface.Method("Lookup").In)
	var v map[string]any
	if err := json.Unmarshal([]byte(text), &v); err != nil {
		t.Fatalf("skeleton is not JSON: %v\n%s", err, text)
	}
	if v["family"] != "inet" {
		t.Errorf("family = %v, want first enum value", v["family"])
	}
	if _, ok := v["ifindex"]; ok {
		t.Error("optional field ifindex should be omitted")
	}
	if got := strings.Join(optional, "; "); got != "ifindex: int; anything: any" {
		t.Errorf("optional = %q", got)
	}
	root := v["root"].(map[string]any)
	if root["name"] != "" {
		t.Errorf("root = %v", root)
	}
	// The skeleton must itself validate.
	if err := Validate(iface, iface.Method("Lookup").In, []byte(text)); err != nil {
		t.Errorf("skeleton does not validate: %v\n%s", err, text)
	}
}

func TestSkeletonEmpty(t *testing.T) {
	iface := parse(t)
	if text, opt := Skeleton(iface, iface.Method("Nothing").In); text != "{}" || opt != nil {
		t.Errorf("got %q %v", text, opt)
	}
}

func TestValidate(t *testing.T) {
	iface := parse(t)
	in := iface.Method("Lookup").In
	base, _ := Skeleton(iface, in)

	tests := []struct {
		name   string
		mutate func(m map[string]any)
		want   string // substring of the first error, "" for valid
	}{
		{"valid", func(map[string]any) {}, ""},
		{"optional null", func(m map[string]any) { m["ifindex"] = nil }, ""},
		{"optional set", func(m map[string]any) { m["ifindex"] = 3 }, ""},
		{"missing required", func(m map[string]any) { delete(m, "name") }, ".name: required field is missing"},
		{"wrong type", func(m map[string]any) { m["flags"] = "x" }, ".flags: expected int, got string"},
		{"fractional int", func(m map[string]any) { m["flags"] = 1.5 }, ".flags: expected int"},
		{"bad enum", func(m map[string]any) { m["family"] = "ipx" }, `"ipx" is not one of`},
		{"unknown field", func(m map[string]any) { m["Name"] = "x" }, `.Name: unknown field (did you mean "name"?)`},
		{"nested array", func(m map[string]any) {
			m["addresses"] = []any{map[string]any{"family": "inet", "address": []any{1, "two"}}}
		}, ".addresses[0].address[1]: expected int"},
		{"map value", func(m map[string]any) { m["labels"] = map[string]any{"k": 1} }, `.labels["k"]: expected string`},
		{"recursive", func(m map[string]any) {
			m["root"] = map[string]any{"name": "a", "children": []any{map[string]any{"name": 1, "children": []any{}}}}
		}, ".root.children[0].name: expected string"},
		{"null required", func(m map[string]any) { m["enabled"] = nil }, ".enabled: must not be null"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m map[string]any
			if err := json.Unmarshal([]byte(base), &m); err != nil {
				t.Fatal(err)
			}
			tt.mutate(m)
			data, _ := json.Marshal(m)
			err := Validate(iface, in, data)
			switch {
			case tt.want == "" && err != nil:
				t.Fatalf("unexpected error: %v", err)
			case tt.want != "" && err == nil:
				t.Fatalf("expected error containing %q", tt.want)
			case tt.want != "" && !strings.Contains(err.Error(), tt.want):
				t.Fatalf("error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestValidateSyntax(t *testing.T) {
	iface := parse(t)
	in := iface.Method("Nothing").In
	for in2, want := range map[string]string{
		"":              "",
		"  ":            "",
		"{}":            "",
		"[]":            "must be a JSON object",
		"{":             "invalid JSON",
		"{\n  \"a\" 1}": "line 2",
		"{} {}":         "unexpected data",
	} {
		err := Validate(iface, in, []byte(in2))
		if want == "" {
			if err != nil {
				t.Errorf("%q: unexpected %v", in2, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: got %v, want %q", in2, err, want)
		}
	}
}
