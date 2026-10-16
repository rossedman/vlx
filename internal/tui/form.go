package tui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/cmdline"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/sanitize"
	"github.com/rossedman/vlx/internal/schema"
)

// fieldKind decides how a parameter is edited.
type fieldKind int

const (
	kindText  fieldKind = iota // string
	kindInt                    // int
	kindFloat                  // float
	kindBool                   // bool: a choice
	kindEnum                   // enum: a choice
	kindJSON                   // structs, arrays, maps, object, any: JSON text
)

const unsetLabel = "(unset)"

// formField is one top-level parameter of a method.
type formField struct {
	name     string
	typ      *idl.Type // as declared, possibly Nullable
	base     *idl.Type // with Nullable and named types resolved
	doc      string
	optional bool
	kind     fieldKind

	input   textinput.Model // text, numbers and JSON
	choices []string        // bool and enum; "(unset)" first when optional
	choice  int
	initial string // the input's starting value, restored when cleared

	err string // set by validate
}

// paramForm edits a method's parameters one field at a time.
type paramForm struct {
	iface  *idl.Interface
	in     *idl.Type
	fields []*formField
	cursor int
	offset int
	width  int
	height int
}

func newParamForm(iface *idl.Interface, in *idl.Type) *paramForm {
	f := &paramForm{iface: iface, in: in}
	if in == nil {
		return f
	}
	for _, fd := range in.Fields {
		f.fields = append(f.fields, newFormField(iface, fd))
	}
	f.focus()
	return f
}

// resolveType strips Nullable and follows named types to their definition.
func resolveType(iface *idl.Interface, t *idl.Type) (base *idl.Type, optional bool) {
	if t.Kind == idl.Nullable {
		optional = true
		t = t.Elem
	}
	for depth := 0; t.Kind == idl.Named && depth < 16; depth++ {
		if iface == nil {
			break
		}
		td := iface.Type(t.Name)
		if td == nil {
			break
		}
		t = td.Type
	}
	return t, optional
}

func newFormField(iface *idl.Interface, fd *idl.Field) *formField {
	base, optional := resolveType(iface, fd.Type)
	ff := &formField{name: fd.Name, typ: fd.Type, base: base, doc: fd.Doc, optional: optional}
	ti := textinput.New()
	ti.Prompt = ""
	ti.CharLimit = 1 << 16
	ti.TextStyle = textStyle
	ti.PlaceholderStyle = faintStyle
	ti.Cursor.Style = lipgloss.NewStyle().Foreground(render.Brand)
	staticCursorInTests(&ti.Cursor)

	switch base.Kind {
	case idl.String:
		ff.kind = kindText
		ti.Placeholder = "text"
	case idl.Int:
		ff.kind = kindInt
		ti.Placeholder = "whole number"
	case idl.Float:
		ff.kind = kindFloat
		ti.Placeholder = "number"
	case idl.Bool:
		ff.kind = kindBool
		ff.choices = []string{"false", "true"}
	case idl.Enum:
		ff.kind = kindEnum
		for _, v := range base.Values {
			ff.choices = append(ff.choices, v.Name)
		}
	default:
		ff.kind = kindJSON
		ti.Placeholder = jsonPlaceholder(base)
		if !optional {
			skel, _ := schema.Skeleton(iface, &idl.Type{Kind: idl.Struct, Fields: []*idl.Field{{Name: "v", Type: fd.Type}}})
			var wrapper map[string]json.RawMessage
			if json.Unmarshal([]byte(skel), &wrapper) == nil {
				ti.SetValue(compactJSON(wrapper["v"]))
			}
		}
	}
	if optional && (ff.kind == kindBool || ff.kind == kindEnum) {
		ff.choices = append([]string{unsetLabel}, ff.choices...)
	}
	if ff.kind == kindText && optional {
		ti.Placeholder = "text (leave empty to omit)"
	}
	if (ff.kind == kindInt || ff.kind == kindFloat) && optional {
		ti.Placeholder += " (empty to omit)"
	}
	ff.input = ti
	ff.initial = ti.Value()
	return ff
}

// args returns the values the form would send as command-line arguments,
// in field order, with the text as typed.
func (f *paramForm) args() []cmdline.Arg {
	var out []cmdline.Arg
	for _, ff := range f.fields {
		if ff.isChoice() {
			if v := ff.choices[ff.choice]; v != unsetLabel {
				out = append(out, cmdline.Arg{Name: ff.name, Value: v})
			}
			continue
		}
		v := ff.input.Value()
		if ff.kind != kindText {
			v = strings.TrimSpace(v)
		}
		if v == "" && (ff.optional || ff.kind != kindText) {
			continue
		}
		out = append(out, cmdline.Arg{Name: ff.name, Value: v})
	}
	return out
}

// loadArgs sets the form from command-line arguments. Fields the
// arguments do not name go back to their starting values. It fails,
// changing nothing, on an unknown name, a repeated name or an impossible
// choice.
func (f *paramForm) loadArgs(args []cmdline.Arg) error {
	given := map[string]string{}
	for _, a := range args {
		if _, dup := given[a.Name]; dup {
			return cmdline.Twice(a.Name)
		}
		given[a.Name] = a.Value
	}
	known := map[string]bool{}
	for _, ff := range f.fields {
		known[ff.name] = true
	}
	for _, a := range args {
		if !known[a.Name] {
			return cmdline.Unknown(a.Name, f.in)
		}
	}
	type setter func()
	var sets []setter
	for _, ff := range f.fields {
		v, ok := given[ff.name]
		switch {
		case ff.isChoice() && !ok:
			sets = append(sets, func() { ff.choice = 0 })
		case ff.isChoice():
			idx, prefixed, n := -1, -1, 0
			for i, c := range ff.choices {
				switch {
				case c == unsetLabel:
				case strings.EqualFold(c, v):
					idx = i
				case v != "" && strings.HasPrefix(strings.ToLower(c), strings.ToLower(v)):
					prefixed = i
					n++
				}
			}
			if idx < 0 && n == 1 {
				idx = prefixed // a unique start is enough while typing
			}
			if idx < 0 {
				return fmt.Errorf("%s=%s: choose one of %s", ff.name, v, strings.Join(ff.choiceValues(), ", "))
			}
			sets = append(sets, func() { ff.choice = idx })
		case !ok:
			sets = append(sets, func() { ff.input.SetValue(ff.initial) })
		default:
			sets = append(sets, func() { ff.input.SetValue(v) })
		}
	}
	for _, set := range sets {
		set()
	}
	return nil
}

// required reports whether any parameter must be given.
func (f *paramForm) required() bool {
	for _, ff := range f.fields {
		if !ff.optional {
			return true
		}
	}
	return false
}

func (ff *formField) choiceValues() []string {
	if ff.optional {
		return ff.choices[1:]
	}
	return ff.choices
}

// lines reports how many lines view needs to show every field.
func (f *paramForm) lines(width int, focused bool) int {
	if len(f.fields) == 0 {
		return 3
	}
	saved := f.offset
	n := strings.Count(f.view(width, 1<<20, focused), "\n") + 1
	f.offset = saved
	return n
}

func jsonPlaceholder(t *idl.Type) string {
	switch t.Kind {
	case idl.Array:
		return "JSON array, e.g. []"
	case idl.Map, idl.Object, idl.Struct:
		return "JSON object, e.g. {}"
	}
	return "any JSON value"
}

func compactJSON(raw []byte) string {
	var b bytes.Buffer
	if err := json.Compact(&b, raw); err != nil {
		return string(raw)
	}
	return b.String()
}

func (ff *formField) isChoice() bool { return ff.kind == kindBool || ff.kind == kindEnum }

// value returns the field's JSON, whether to include it, and a problem.
func (ff *formField) value() (raw json.RawMessage, include bool, problem string) {
	if ff.isChoice() {
		v := ff.choices[ff.choice]
		if v == unsetLabel {
			return nil, false, ""
		}
		if ff.kind == kindBool {
			return json.RawMessage(v), true, ""
		}
		q, _ := json.Marshal(v)
		return q, true, ""
	}
	s := ff.input.Value()
	switch ff.kind {
	case kindText:
		if s == "" && ff.optional {
			return nil, false, ""
		}
		q, _ := json.Marshal(s)
		return q, true, ""
	case kindInt:
		s = strings.TrimSpace(s)
		if s == "" {
			if ff.optional {
				return nil, false, ""
			}
			return nil, false, "required: enter a whole number"
		}
		if _, err := strconv.ParseInt(s, 10, 64); err != nil {
			if _, uerr := strconv.ParseUint(s, 10, 64); uerr != nil {
				return nil, false, "not a whole number"
			}
		}
		return json.RawMessage(s), true, ""
	case kindFloat:
		s = strings.TrimSpace(s)
		if s == "" {
			if ff.optional {
				return nil, false, ""
			}
			return nil, false, "required: enter a number"
		}
		if _, err := strconv.ParseFloat(s, 64); err != nil || !json.Valid([]byte(s)) {
			return nil, false, "not a number"
		}
		return json.RawMessage(s), true, ""
	default:
		s = strings.TrimSpace(s)
		if s == "" {
			if ff.optional {
				return nil, false, ""
			}
			return nil, false, "required: enter " + jsonPlaceholder(ff.base)
		}
		if !json.Valid([]byte(s)) {
			return nil, false, "not valid JSON"
		}
		return json.RawMessage(s), true, ""
	}
}

// JSON returns the parameters as an indented JSON object, in field order.
// Fields with problems are left out; validate reports them.
func (f *paramForm) JSON() string {
	var b strings.Builder
	n := 0
	for _, ff := range f.fields {
		raw, include, problem := ff.value()
		if !include || problem != "" {
			continue
		}
		if n == 0 {
			b.WriteString("{\n")
		} else {
			b.WriteString(",\n")
		}
		k, _ := json.Marshal(ff.name)
		b.WriteString("  " + string(k) + ": " + compactJSON(raw))
		n++
	}
	if n == 0 {
		return "{}"
	}
	b.WriteString("\n}")
	return b.String()
}

// validate checks every field and the assembled object against the IDL,
// attaching problems to their fields. It returns the first problem.
func (f *paramForm) validate(in *idl.Type) error {
	var first error
	for _, ff := range f.fields {
		ff.err = ""
		if _, _, problem := ff.value(); problem != "" {
			ff.err = problem
			if first == nil {
				first = fmt.Errorf("%s: %s", ff.name, problem)
			}
		}
	}
	if first != nil {
		return first
	}
	err := schema.Validate(f.iface, in, []byte(f.JSON()))
	var es schema.Errors
	if errors.As(err, &es) {
		for _, e := range es {
			name, rest := splitPath(e.Path)
			for _, ff := range f.fields {
				if ff.name == name && ff.err == "" {
					ff.err = strings.TrimPrefix(rest+": "+e.Msg, ": ")
				}
			}
		}
	}
	return err
}

// splitPath splits ".addresses[2].family" into "addresses" and
// "[2].family".
func splitPath(p string) (string, string) {
	p = strings.TrimPrefix(p, ".")
	i := strings.IndexAny(p, ".[")
	if i < 0 {
		return p, ""
	}
	return p[:i], p[i:]
}

// load sets the fields from a JSON object. It fails, changing nothing, if
// the object has keys or values the form cannot represent.
func (f *paramForm) load(data string) error {
	data = strings.TrimSpace(data)
	if data == "" {
		data = "{}"
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return errors.New("the JSON is not a valid object")
	}
	known := map[string]*formField{}
	for _, ff := range f.fields {
		known[ff.name] = ff
	}
	for k := range obj {
		if known[k] == nil {
			return fmt.Errorf("%q is not a parameter of this method", k)
		}
	}
	type setter func()
	var sets []setter
	for _, ff := range f.fields {
		raw, present := obj[ff.name]
		isNull := !present || string(bytes.TrimSpace(raw)) == "null"
		switch {
		case ff.isChoice():
			idx := 0
			if !isNull {
				var v any
				_ = json.Unmarshal(raw, &v)
				want := fmt.Sprint(v)
				if _, ok := v.(bool); !ok && ff.kind == kindBool {
					return fmt.Errorf("%s must be true or false", ff.name)
				}
				idx = -1
				for i, c := range ff.choices {
					if c == want && c != unsetLabel {
						idx = i
					}
				}
				if idx < 0 {
					return fmt.Errorf("%s: %s is not one of the choices", ff.name, compactJSON(raw))
				}
			}
			sets = append(sets, func() { ff.choice = idx })
		case isNull:
			sets = append(sets, func() { ff.input.SetValue("") })
		case ff.kind == kindText:
			var s string
			if json.Unmarshal(raw, &s) != nil {
				return fmt.Errorf("%s must be a string", ff.name)
			}
			if s == "" && ff.optional {
				return fmt.Errorf("%s is an empty optional string, which only JSON mode can express", ff.name)
			}
			sets = append(sets, func() { ff.input.SetValue(s) })
		default:
			v := compactJSON(raw)
			sets = append(sets, func() { ff.input.SetValue(v) })
		}
	}
	for _, s := range sets {
		s()
	}
	return nil
}

func (f *paramForm) current() *formField {
	if f.cursor < 0 || f.cursor >= len(f.fields) {
		return nil
	}
	return f.fields[f.cursor]
}

// focus gives the keyboard to the current field.
func (f *paramForm) focus() tea.Cmd {
	var cmd tea.Cmd
	for i, ff := range f.fields {
		if i == f.cursor && !ff.isChoice() {
			cmd = ff.input.Focus()
			ff.input.CursorEnd()
		} else {
			ff.input.Blur()
		}
	}
	return cmd
}

func (f *paramForm) blur() {
	for _, ff := range f.fields {
		ff.input.Blur()
	}
}

func (f *paramForm) move(d int) tea.Cmd {
	if len(f.fields) == 0 {
		return nil
	}
	f.cursor = clamp(f.cursor+d, 0, len(f.fields)-1)
	return f.focus()
}

// update handles a key on the form. changed reports whether a value
// changed.
func (f *paramForm) update(msg tea.KeyMsg) (cmd tea.Cmd, changed bool) {
	ff := f.current()
	switch msg.String() {
	case "up", "ctrl+p", "shift+up":
		return f.move(-1), false
	case "down", "ctrl+n", "enter", "shift+down":
		return f.move(1), false
	}
	if ff == nil {
		return nil, false
	}
	if ff.isChoice() {
		switch msg.String() {
		case "left":
			ff.choice = (ff.choice - 1 + len(ff.choices)) % len(ff.choices)
			return nil, true
		case "right", " ":
			ff.choice = (ff.choice + 1) % len(ff.choices)
			return nil, true
		case "backspace", "delete":
			if ff.optional {
				ff.choice = 0
				return nil, true
			}
		}
		// Typing the start of a choice selects it.
		if msg.Type == tea.KeyRunes {
			prefix := strings.ToLower(string(msg.Runes))
			for i := range ff.choices {
				j := (ff.choice + 1 + i) % len(ff.choices)
				if strings.HasPrefix(ff.choices[j], prefix) {
					ff.choice = j
					return nil, true
				}
			}
		}
		return nil, false
	}
	before := ff.input.Value()
	ff.input, cmd = ff.input.Update(msg)
	return cmd, ff.input.Value() != before
}

// forward passes non-key messages (cursor blinks) to the focused input.
func (f *paramForm) forward(msg tea.Msg) tea.Cmd {
	ff := f.current()
	if ff == nil || ff.isChoice() {
		return nil
	}
	var cmd tea.Cmd
	ff.input, cmd = ff.input.Update(msg)
	return cmd
}

// view renders the form in width×height, keeping the current field
// visible.
func (f *paramForm) view(width, height int, focused bool) string {
	if len(f.fields) == 0 {
		return mutedStyle.Render("This method takes no parameters.") + "\n\n" +
			mutedStyle.Render("Press ") + brandStyle.Render("ctrl+s") + mutedStyle.Render(" to call it.")
	}
	nameW, typeW := 0, 0
	for _, ff := range f.fields {
		nameW = max(nameW, ansi.StringWidth(ff.name))
		typeW = max(typeW, ansi.StringWidth(ff.typ.String()))
	}
	nameW = min(nameW, max(width/4, 8))
	typeW = min(typeW, max(width/5, 6))
	valueW := max(width-2-nameW-2-typeW-2, 8)

	var lines []string
	start, end := 0, 0
	for i, ff := range f.fields {
		cur := i == f.cursor
		if cur {
			start = len(lines)
		}
		marker := "  "
		nameStyle := textStyle
		if ff.optional {
			if _, include, _ := ff.value(); !include {
				nameStyle = mutedStyle
			}
		}
		if cur && focused {
			marker = brandStyle.Render("▸ ")
			nameStyle = brandStyle
		}
		name := padRight(ansi.Truncate(sanitize.Line(ff.name), nameW, "…"), nameW)
		typ := padRight(ansi.Truncate(sanitize.Line(ff.typ.String()), typeW, "…"), typeW)
		line := marker + nameStyle.Render(name) + "  " + render.TypeStyle.Render(typ) + "  " + f.renderValue(ff, valueW, cur && focused)
		lines = append(lines, ansi.Truncate(line, width, "…"))

		indent := strings.Repeat(" ", 2+nameW+2)
		docW := max(width-len(indent), 10)
		if ff.err != "" {
			lines = append(lines, indent+errStyle.Render(ansi.Truncate("✗ "+sanitize.Line(ff.err), docW, "…")))
		}
		if cur && focused {
			if ff.doc != "" {
				doc := lipgloss.NewStyle().Width(docW).Foreground(render.Muted).Render(sanitize.Text(ff.doc))
				for _, l := range strings.Split(doc, "\n") {
					lines = append(lines, indent+l)
				}
			}
			if h := ff.hint(); h != "" {
				wrapped := lipgloss.NewStyle().Width(docW).Foreground(render.Faint).Render(h)
				for _, l := range strings.Split(wrapped, "\n") {
					lines = append(lines, indent+l)
				}
			}
		}
		if cur {
			end = len(lines)
		}
	}
	// Scroll so the current field and its notes are visible.
	if start < f.offset {
		f.offset = start
	}
	if end > f.offset+height {
		f.offset = end - height
	}
	if start < f.offset {
		f.offset = start // a field's own line beats its notes
	}
	f.offset = clamp(f.offset, 0, max(len(lines)-height, 0))
	lines = lines[f.offset:min(f.offset+height, len(lines))]
	return strings.Join(lines, "\n")
}

func (f *paramForm) renderValue(ff *formField, width int, active bool) string {
	if ff.isChoice() {
		v := ff.choices[ff.choice]
		st := render.StringStyle
		switch {
		case v == unsetLabel:
			st = faintStyle
		case ff.kind == kindBool:
			st = render.BoolStyle
		}
		if active {
			return brandStyle.Render("‹ ") + st.Bold(true).Render(v) + brandStyle.Render(" ›")
		}
		return "  " + st.Render(v)
	}
	ff.input.Width = max(width-3, 4)
	prefix := "  "
	if active {
		prefix = brandStyle.Render("› ")
	}
	return prefix + ff.input.View()
}

func (ff *formField) hint() string {
	switch ff.kind {
	case kindBool:
		return "←/→ or space to switch"
	case kindEnum:
		return "←/→ to choose: " + strings.Join(ff.choiceValues(), " · ")
	case kindJSON:
		return "JSON value · ctrl+j edits all parameters as JSON"
	case kindInt, kindFloat:
		if ff.optional {
			return "leave empty to omit"
		}
		return "a number"
	default:
		if ff.optional {
			return "leave empty to omit"
		}
		return ""
	}
}

func padRight(s string, w int) string {
	if d := w - ansi.StringWidth(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}
