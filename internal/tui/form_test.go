package tui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"
)

const formIDL = `
interface com.example.Form

type Color (red, green, blue)
type Point (x: int, y: int)

method Do(
  # Who to greet.
  name: string,
  count: int,
  ratio: ?float,
  color: Color,
  shade: ?Color,
  loud: bool,
  quiet: ?bool,
  label: ?string,
  where: Point,
  tags: ?[]string
) -> ()
`

func testForm(t *testing.T) (*paramForm, *idl.Type) {
	t.Helper()
	iface, err := idl.Parse(formIDL)
	if err != nil {
		t.Fatal(err)
	}
	in := iface.Method("Do").In
	return newParamForm(iface, in), in
}

func field(f *paramForm, name string) *formField {
	for _, ff := range f.fields {
		if ff.name == name {
			return ff
		}
	}
	return nil
}

func TestFormDefaults(t *testing.T) {
	f, in := testForm(t)
	kinds := map[string]fieldKind{
		"name": kindText, "count": kindInt, "ratio": kindFloat, "color": kindEnum,
		"shade": kindEnum, "loud": kindBool, "quiet": kindBool, "label": kindText,
		"where": kindJSON, "tags": kindJSON,
	}
	for name, k := range kinds {
		if got := field(f, name).kind; got != k {
			t.Errorf("%s kind = %v, want %v", name, got, k)
		}
	}
	if field(f, "shade").choices[0] != unsetLabel || field(f, "color").choices[0] != "red" {
		t.Error("optional choices must start unset, required ones with the first value")
	}
	if got := field(f, "where").input.Value(); got != `{"x":0,"y":0}` {
		t.Errorf("struct skeleton = %q", got)
	}
	// count is required and empty, so the form is invalid until filled.
	if err := f.validate(in); err == nil || !strings.Contains(err.Error(), "count") {
		t.Fatalf("validate = %v", err)
	}
	field(f, "count").input.SetValue("3")
	if err := f.validate(in); err != nil {
		t.Fatalf("validate = %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(f.JSON()), &got); err != nil {
		t.Fatal(err)
	}
	want := `{"color":"red","count":3,"loud":false,"name":"","where":{"x":0,"y":0}}`
	if b, _ := json.Marshal(got); string(b) != want {
		t.Errorf("JSON = %s, want %s", b, want)
	}
}

func TestFormKeys(t *testing.T) {
	f, in := testForm(t)
	key := func(s string) {
		var msg tea.KeyMsg
		switch s {
		case "down", "up", "left", "right", "enter", "backspace":
			msg = map[string]tea.KeyMsg{
				"down": {Type: tea.KeyDown}, "up": {Type: tea.KeyUp}, "left": {Type: tea.KeyLeft},
				"right": {Type: tea.KeyRight}, "enter": {Type: tea.KeyEnter}, "backspace": {Type: tea.KeyBackspace},
			}[s]
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
		}
		f.update(msg)
	}
	for _, r := range "Ada" {
		key(string(r))
	}
	key("enter") // to count
	key("4")
	key("2")
	key("down") // ratio
	key("down") // color
	key("right")
	key("down") // shade
	key("b")    // type-ahead picks blue
	key("down") // loud
	key(" ")
	if err := f.validate(in); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal([]byte(f.JSON()), &got)
	if got["name"] != "Ada" || got["count"] != float64(42) || got["color"] != "green" || got["shade"] != "blue" || got["loud"] != true {
		t.Fatalf("JSON = %s", f.JSON())
	}
	if _, ok := got["ratio"]; ok {
		t.Error("empty optional number was included")
	}
	key("up") // shade
	key("backspace")
	if field(f, "shade").choices[field(f, "shade").choice] != unsetLabel {
		t.Error("backspace did not unset an optional choice")
	}
}

func TestFormFieldErrors(t *testing.T) {
	f, in := testForm(t)
	field(f, "count").input.SetValue("1.5")
	field(f, "where").input.SetValue(`{"x":"no","y":0}`)
	if err := f.validate(in); err == nil {
		t.Fatal("expected an error")
	}
	if field(f, "count").err != "not a whole number" {
		t.Errorf("count err = %q", field(f, "count").err)
	}
	field(f, "count").input.SetValue("1")
	_ = f.validate(in)
	if !strings.Contains(field(f, "where").err, ".x: expected int") {
		t.Errorf("nested error not attached to its field: %q", field(f, "where").err)
	}
}

func TestFormLoadRoundTrip(t *testing.T) {
	f, in := testForm(t)
	src := `{"name":"Bo","count":7,"ratio":0.5,"color":"blue","shade":"red","loud":true,"quiet":false,"label":"x","where":{"x":1,"y":2},"tags":["a"]}`
	if err := f.load(src); err != nil {
		t.Fatal(err)
	}
	if err := f.validate(in); err != nil {
		t.Fatal(err)
	}
	var a, b map[string]any
	_ = json.Unmarshal([]byte(src), &a)
	_ = json.Unmarshal([]byte(f.JSON()), &b)
	ab, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if string(ab) != string(bb) {
		t.Errorf("round trip:\n%s\n%s", ab, bb)
	}
	for _, bad := range []string{`{"nope":1}`, `{"color":"pink"}`, `{"loud":"yes"}`, `{"name":5}`, `{"label":""}`, `[`} {
		before := f.JSON()
		if err := f.load(bad); err == nil {
			t.Errorf("load(%s) succeeded", bad)
		}
		if f.JSON() != before {
			t.Errorf("failed load(%s) changed the form", bad)
		}
	}
}

func TestFormViewFits(t *testing.T) {
	f, _ := testForm(t)
	f.cursor = 8
	f.focus()
	v := f.view(50, 6, true)
	lines := strings.Split(v, "\n")
	if len(lines) > 6 {
		t.Fatalf("%d lines", len(lines))
	}
	for _, l := range lines {
		if w := ansi.StringWidth(l); w > 50 {
			t.Errorf("line %d wide: %q", w, ansi.Strip(l))
		}
	}
	if !strings.Contains(ansi.Strip(v), "where") {
		t.Errorf("current field scrolled out of view:\n%s", ansi.Strip(v))
	}
}
