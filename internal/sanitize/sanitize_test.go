package sanitize

import "testing"

func TestLine(t *testing.T) {
	tests := map[string]string{
		"plain":                      "plain",
		"io.systemd.Hostname":        "io.systemd.Hostname",
		"two\nlines\tand tab":        "two lines and tab",
		"\x1b[31mred\x1b[0m":         "[31mred[0m",
		"title\x1b]0;pwned\x07":      "title]0;pwned",
		"clip\x1b]52;c;ZXZpbA==\x07": "clip]52;c;ZXZpbA==",
		"c1\u009b31m":                "c131m",
		"over\rwrite":                "overwrite",
		"bidi\u202eevil":             "bidievil",
		"héllo wörld ✓":              "héllo wörld ✓",
		"bad\xffbyte":                "bad�byte",
	}
	for in, want := range tests {
		if got := Line(in); got != want {
			t.Errorf("Line(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTextKeepsNewlines(t *testing.T) {
	in := "first line\n\tsecond\x1b[2J line"
	want := "first line\n\tsecond[2J line"
	if got := Text(in); got != want {
		t.Errorf("Text(%q) = %q, want %q", in, got, want)
	}
}
