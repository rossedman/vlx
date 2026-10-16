package render

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestJSONPlain(t *testing.T) {
	in := `{"z":1,"a":{"password":"hunter2","list":[1,true,null,"x"],"empty":{},"none":[]},"apiToken":null,"esc":"\u001b[2J"}`
	got := JSON([]byte(in), JSONOptions{Redact: NewRedactor(DefaultRedact)})
	want := `{
  "z": 1,
  "a": {
    "password": «redacted»,
    "list": [
      1,
      true,
      null,
      "x"
    ],
    "empty": {},
    "none": []
  },
  "apiToken": null,
  "esc": "\u001b[2J"
}`
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestJSONNoRedact(t *testing.T) {
	got := JSON([]byte(`{"apiToken":"abc"}`), JSONOptions{})
	if !strings.Contains(got, `"abc"`) {
		t.Errorf("got %s", got)
	}
}

func TestJSONInvalid(t *testing.T) {
	if got := JSON([]byte("{bad\x1b"), JSONOptions{}); got != "{bad" {
		t.Errorf("got %q", got)
	}
}

func TestJSONColorNoEscapesFromData(t *testing.T) {
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	got := JSON([]byte(`{"k":"v"}`), JSONOptions{Color: true})
	if !strings.Contains(got, "\x1b[") {
		t.Fatalf("expected colour codes in %q", got)
	}
}

func TestIDL(t *testing.T) {
	src := "# doc\x1b]0;x\x07\ninterface a.b\nmethod Foo(x: int) -> ()"
	plain := IDL(src, false)
	if strings.ContainsRune(plain, 0x1b) {
		t.Errorf("escape survived: %q", plain)
	}
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(termenv.Ascii)
	colored := IDL(src, true)
	if !strings.Contains(colored, "Foo") || !strings.Contains(colored, "\x1b[") {
		t.Errorf("got %q", colored)
	}
}

func TestRedactorPatterns(t *testing.T) {
	r := NewRedactor(DefaultRedact)
	for key, want := range map[string]bool{
		"password": true, "hashedPassword": true, "apiToken": true, "privileged": true,
		"PrivateKey": true, "tokenizer": true, "name": false, "secretive": true, "user": false,
	} {
		if got := r.Match(key); got != want {
			t.Errorf("Match(%q) = %v", key, got)
		}
	}
	var nilR *Redactor
	if nilR.Match("password") {
		t.Error("nil redactor matched")
	}
}

func TestCallCommands(t *testing.T) {
	v, c := CallCommands("/run/x y", "a.B.C", `{"name":"it's"}`, true, false)
	if v != `vlx call --more '/run/x y' a.B.C '{"name":"it'\''s"}'` {
		t.Errorf("vlx = %s", v)
	}
	if c != `varlinkctl call --more '/run/x y' a.B.C '{"name":"it'\''s"}'` {
		t.Errorf("varlinkctl = %s", c)
	}
	v, c = CallCommands("/run/s", "a.B.C", "{}", false, true)
	if v != "vlx call --oneway /run/s a.B.C" || c != "varlinkctl call --oneway /run/s a.B.C '{}'" {
		t.Errorf("got %s | %s", v, c)
	}
}
