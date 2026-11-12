package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/render"
)

// FuzzCleanValue checks that cleaning any reply yields encodable JSON with
// no control characters and no value left under a redacted key.
func FuzzCleanValue(f *testing.F) {
	for _, seed := range []string{
		`{"a":"b"}`, `{"password":"x","n":{"token":[1,2]}}`, `["\u001b[31m",null,1e400]`,
		`{"k\u0007":"\u202e"}`, `"plain"`, `12345678901234567890`, `{`, ``, `[[[[[[[]]]]]]]`,
	} {
		f.Add([]byte(seed))
	}
	s := New(Options{})
	f.Fuzz(func(t *testing.T, raw []byte) {
		v := s.cleanValue(raw)
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("cleaned value does not encode: %v", err)
		}
		var back any
		if err := json.Unmarshal(b, &back); err != nil {
			t.Fatalf("cleaned value is not JSON: %s", b)
		}
		walk(t, back)
	})
}

// walk fails on control characters in strings or keys and on values left
// under keys the default redactor matches.
func walk(t *testing.T, v any) {
	t.Helper()
	switch x := v.(type) {
	case string:
		if strings.ContainsAny(x, "\x1b\x00\x07\u202e") {
			t.Fatalf("control character left in %q", x)
		}
	case []any:
		for _, e := range x {
			walk(t, e)
		}
	case map[string]any:
		r := render.NewRedactor(render.DefaultRedact)
		for k, e := range x {
			walk(t, k)
			if r.Match(k) && e != nil && e != render.Redacted {
				t.Fatalf("value under %q not redacted: %v", k, e)
			}
			walk(t, e)
		}
	}
}

// FuzzCallParams checks that the call path never panics on arbitrary
// arguments and either rejects them or produces params that are a JSON
// object matching the method.
func FuzzCallParams(f *testing.F) {
	for _, seed := range []string{
		`{"service":"a","method":"b"}`, `{"service":"a","method":"b","params":{"all":{}}}`,
		`{"service":"a","method":"b","params":null}`, `{"service":"a","method":"b","params":[1]}`,
		`{"service":"a","method":"b","stream":{"max_replies":-5}}`, `{"params":{"tag":"\u0000"}}`, `nope`,
	} {
		f.Add([]byte(seed))
	}
	iface, err := parseOnce()
	if err != nil {
		f.Fatal(err)
	}
	m := iface.Method("Get")
	f.Fuzz(func(t *testing.T, args []byte) {
		in, err := decodeCallInput(args)
		if err != nil {
			if !json.Valid([]byte(err.Error())) {
				t.Fatalf("error is not JSON: %s", err.Error())
			}
			return
		}
		params, err := callParams(iface, m, in.Params)
		if err != nil {
			return
		}
		var obj map[string]any
		if err := json.Unmarshal(params, &obj); err != nil {
			t.Fatalf("accepted params are not an object: %s", params)
		}
		if _, ok := obj["all"]; !ok {
			t.Fatalf("accepted params without the required field: %s", params)
		}
	})
}

// parseOnce parses schemaIDL for the fuzz targets.
func parseOnce() (*idl.Interface, error) { return idl.Parse(schemaIDL) }
