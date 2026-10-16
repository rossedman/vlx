package schema

import (
	"strings"
	"testing"

	"github.com/rossedman/sdvarlink/idl"
)

func TestValidateWithoutInterface(t *testing.T) {
	// Named types cannot be resolved without an interface; the service
	// decides.
	in, err := idl.ParseType("(a: Unknown, b: ?float, c: object, d: any)")
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(nil, in, []byte(`{"a": 1, "b": 1.5, "c": {}, "d": [1]}`)); err != nil {
		t.Errorf("unexpected %v", err)
	}
	for params, want := range map[string]string{
		`{"a": 1, "b": "x", "c": {}}`: ".b: expected float, got string",
		`{"a": 1, "c": []}`:           ".c: expected object, got array",
		`{"a": 1, "c": true}`:         "got bool",
	} {
		err := Validate(nil, in, []byte(params))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: %v, want %q", params, err, want)
		}
	}
	if err := Validate(nil, nil, []byte(`{"x":1}`)); err == nil {
		t.Error("nil type accepts unknown fields")
	}
	text, opt := Skeleton(nil, in)
	if !strings.Contains(text, `"a": null`) || len(opt) != 1 {
		t.Errorf("skeleton %s %v", text, opt)
	}
	if text, _ := Skeleton(nil, nil); text != "{}" {
		t.Error("nil skeleton")
	}
}

func TestShapes(t *testing.T) {
	in, err := idl.ParseType("(e: (x, y), n: ?(p: int), m: [string]bool, l: []int, s: (q: ?int), u: int)")
	if err != nil {
		t.Fatal(err)
	}
	text, _ := Skeleton(nil, in)
	for _, want := range []string{`"e": "x"`, `"m": {}`, `"l": []`, `"s": {}`, `"u": 0`} {
		if !strings.Contains(text, want) {
			t.Errorf("skeleton lacks %s:\n%s", want, text)
		}
	}
	for params, want := range map[string]string{
		`{"e":1,"m":{},"l":[],"s":{},"u":0}`:                      "expected one of",
		`{"e":"x","m":[],"l":[],"s":{},"u":0}`:                    "expected object (map)",
		`{"e":"x","m":{},"l":{},"s":{},"u":0}`:                    "expected array",
		`{"e":"x","m":{},"l":[],"s":1,"u":0}`:                     "expected object, got number",
		`{"e":"x","m":{},"l":[],"s":{},"u":18446744073709551615}`: "",
		`{"e":"x","m":{},"l":[],"s":{},"u":99999999999999999999}`: "expected int",
		`{"e":"x","m":{},"l":[],"s":{},"u":true}`:                 "expected int, got bool",
	} {
		err := Validate(nil, in, []byte(params))
		switch {
		case want == "" && err != nil:
			t.Errorf("%s: unexpected %v", params, err)
		case want != "" && (err == nil || !strings.Contains(err.Error(), want)):
			t.Errorf("%s: %v, want %q", params, err, want)
		}
	}
}

func TestErrorsFormatting(t *testing.T) {
	if (&Error{Msg: "m"}).Error() != "m" {
		t.Error("no path")
	}
	if (Errors{}).Error() != "no errors" {
		t.Error("empty")
	}
	es := Errors{{Path: ".a", Msg: "x"}, {Path: ".b", Msg: "y"}}
	if es.Error() != ".a: x (and 1 more)" {
		t.Errorf("got %q", es.Error())
	}
}

func TestDeepRecursionStops(t *testing.T) {
	iface, err := idl.Parse("interface a.b\ntype Loop (next: Loop)\nmethod M(l: Loop) -> ()")
	if err != nil {
		t.Fatal(err)
	}
	text, _ := Skeleton(iface, iface.Method("M").In)
	if !strings.Contains(text, "null") {
		t.Errorf("recursive skeleton did not stop:\n%s", text)
	}
	_ = Validate(iface, iface.Method("M").In, []byte(text))
}
