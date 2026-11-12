package mcpserver

import (
	"encoding/json"
	"strings"
	"testing"
)

// BenchmarkCleanValue measures sanitising and redacting a typical reply.
func BenchmarkCleanValue(b *testing.B) {
	s := New(Options{})
	raw := json.RawMessage(`{"items":[` + strings.TrimSuffix(strings.Repeat(`{"name":"unit\u001b[1m.service","state":"active","token":"x","n":12345},`, 100), ",") + `]}`)
	b.ReportAllocs()
	for b.Loop() {
		s.cleanValue(raw)
	}
}

// BenchmarkJSONSchema measures schema generation for a method with nested
// and recursive types.
func BenchmarkJSONSchema(b *testing.B) {
	iface, err := parseOnce()
	if err != nil {
		b.Fatal(err)
	}
	in := iface.Method("Get").In
	b.ReportAllocs()
	for b.Loop() {
		JSONSchema(iface, in)
	}
}
