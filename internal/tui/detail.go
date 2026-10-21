package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/sanitize"
)

// detailRow is the docs view of an interface, method, type or error, as
// shown on a detail page.
func (m *Model) detailRow(r item, width int) string {
	var b strings.Builder
	switch r.kind {
	case itemIface:
		if r.iface.Parsed == nil {
			b.WriteString("\n" + errStyle.Render("✗ ") + wrap(errStyle.Render(sanitize.Line(fmt.Sprint(r.iface.Err))), width-2))
			return b.String()
		}
		p := r.iface.Parsed
		if p.Doc != "" {
			b.WriteString("\n" + wrap(docStyle.Render(sanitize.Text(p.Doc)), width) + "\n")
		}
		b.WriteString("\n" + mutedStyle.Render(counts(p)) + "\n")
		if len(p.Methods) > 0 {
			b.WriteString(heading("Methods") + "\n")
			for _, mt := range p.Methods {
				v := m.classifier.Classify(r.iface.Name + "." + mt.Name)
				b.WriteString(classTag(v.Class) + strings.Repeat(" ", 7-len(v.Class.String())) + " " + textStyle.Render(sanitize.Line(mt.Name)))
				if mt.More != idl.MoreNone {
					b.WriteString(mutedStyle.Render("  ⌁ streams"))
				}
				b.WriteByte('\n')
			}
		}
		b.WriteString("\n" + mutedStyle.Render("Press ctrl+d for the IDL source."))
	case itemMethod:
		m.detailMethod(&b, r, width)
	case itemType:
		if r.typ.Doc != "" {
			b.WriteString("\n" + wrap(docStyle.Render(sanitize.Text(r.typ.Doc)), width) + "\n")
		}
		b.WriteString(heading(kindLabel(r.typ.Type)) + "\n")
		writeTypeBody(&b, r.typ.Type, width)
		if users := usersOf(r.iface.Parsed, r.typ.Name); len(users) > 0 {
			b.WriteString(heading("Used by") + "\n")
			for _, u := range users {
				b.WriteString("• " + textStyle.Render(sanitize.Line(u)) + "\n")
			}
		}
	case itemError:
		if r.err.Doc != "" {
			b.WriteString("\n" + wrap(docStyle.Render(sanitize.Text(r.err.Doc)), width) + "\n")
		}
		b.WriteString(heading("Parameters") + "\n")
		writeFields(&b, r.err.Params, width)
	}
	return b.String()
}

// kindLabel heads the body of a type: "Values" for an enum, "Fields"
// otherwise.
func kindLabel(t *idl.Type) string {
	if t != nil && t.Kind == idl.Enum {
		return "Values"
	}
	return "Fields"
}

// detailMethod writes a method's docs: whether it streams, its parameters,
// returns, the types it uses, its interface's errors and its safety class.
func (m *Model) detailMethod(b *strings.Builder, r item, width int) {
	mt := r.method

	r.verdict = m.classifier.Classify(r.qualified())
	tags := []string{classTag(r.verdict.Class)}
	switch mt.More {
	case idl.MoreRequired:
		tags = append(tags, brandStyle.Render("⌁ streams")+mutedStyle.Render(" (requires more)"))
	case idl.MoreSupported:
		tags = append(tags, brandStyle.Render("⌁ can stream")+mutedStyle.Render(" (supports more)"))
	default:
		tags = append(tags, mutedStyle.Render("single reply"))
	}
	b.WriteString(strings.Join(tags, mutedStyle.Render(" · ")) + "\n")

	if mt.Doc != "" {
		b.WriteString("\n" + wrap(docStyle.Render(sanitize.Text(mt.Doc)), width) + "\n")
	}
	b.WriteString(heading("Parameters") + "\n")
	writeFields(b, mt.In, width)
	b.WriteString(heading("Returns") + "\n")
	writeFields(b, mt.Out, width)

	if used := namedTypes(r.iface.Parsed, mt); len(used) > 0 {
		b.WriteString(heading("Types") + "\n")
		for _, td := range used {
			b.WriteString(mutedStyle.Render("type ") + render.TypeStyle.Bold(true).Render(sanitize.Line(td.Name)) + "\n")
			writeTypeBody(b, td.Type, width)
		}
	}
	if p := r.iface.Parsed; p != nil && len(p.Errors) > 0 {
		b.WriteString(heading("Errors in this interface") + "\n")
		names := make([]string, len(p.Errors))
		for i, e := range p.Errors {
			names[i] = sanitize.Line(e.Name)
		}
		b.WriteString(wrap(errStyle.Render(strings.Join(names, ", ")), width) + "\n")
	}

	b.WriteString(heading("Safety") + "\n")
	b.WriteString(wrap(classTag(r.verdict.Class)+mutedStyle.Render(" — "+r.verdict.Reason), width) + "\n")
	d := m.policy.Decide(r.qualified(), r.verdict)
	if !d.Allowed {
		b.WriteString(wrap(warnStyle.Render("Blocked in read-only mode: you can prepare this call but not send it."), width) + "\n")
	}
}

// writeFields renders the fields of a struct type as "name  type" lines
// with docs beneath.
func writeFields(b *strings.Builder, t *idl.Type, width int) {
	if t == nil || len(t.Fields) == 0 {
		b.WriteString(mutedStyle.Render("  (none)") + "\n")
		return
	}
	nameW := 0
	for _, f := range t.Fields {
		nameW = max(nameW, len(f.Name))
	}
	nameW = min(nameW, max(width/2, 8))
	for _, f := range t.Fields {
		name := sanitize.Line(f.Name)
		pad := strings.Repeat(" ", max(nameW-len(name), 0))
		b.WriteString("  " + render.KeyStyle.Render(name) + pad + "  " + render.TypeStyle.Render(sanitize.Line(f.Type.String())))
		if f.Type.Kind == idl.Nullable {
			b.WriteString(mutedStyle.Render("  optional"))
		}
		b.WriteByte('\n')
		if f.Doc != "" {
			doc := lipgloss.NewStyle().Width(max(width-4, 10)).Foreground(render.Muted).Render(sanitize.Text(f.Doc))
			for _, l := range strings.Split(doc, "\n") {
				b.WriteString("    " + l + "\n")
			}
		}
	}
}

// writeTypeBody writes an enum's values, or the fields of any other type.
func writeTypeBody(b *strings.Builder, t *idl.Type, width int) {
	if t == nil {
		return
	}
	if t.Kind == idl.Enum {
		for _, v := range t.Values {
			b.WriteString("  " + render.StringStyle.Render(sanitize.Line(v.Name)))
			if v.Doc != "" {
				b.WriteString(mutedStyle.Render("  " + sanitize.Line(v.Doc)))
			}
			b.WriteByte('\n')
		}
		return
	}
	writeFields(b, t, width)
}

// namedTypes returns the type definitions a method refers to, directly or
// through other types, in order of first use.
func namedTypes(iface *idl.Interface, mt *idl.Method) []*idl.TypeDef {
	if iface == nil {
		return nil
	}
	var out []*idl.TypeDef
	seen := map[string]bool{}
	var walk func(t *idl.Type)
	walk = func(t *idl.Type) {
		if t == nil || len(out) > 32 {
			return
		}
		switch t.Kind {
		case idl.Named:
			if seen[t.Name] {
				return
			}
			seen[t.Name] = true
			if td := iface.Type(t.Name); td != nil {
				out = append(out, td)
				walk(td.Type)
			}
		case idl.Array, idl.Map, idl.Nullable:
			walk(t.Elem)
		case idl.Struct:
			for _, f := range t.Fields {
				walk(f.Type)
			}
		}
	}
	walk(mt.In)
	walk(mt.Out)
	return out
}

// usersOf lists the methods, types and errors that refer to the named type.
func usersOf(iface *idl.Interface, name string) []string {
	if iface == nil {
		return nil
	}
	var refers func(t *idl.Type, depth int) bool
	refers = func(t *idl.Type, depth int) bool {
		if t == nil || depth > 16 {
			return false
		}
		switch t.Kind {
		case idl.Named:
			return t.Name == name
		case idl.Array, idl.Map, idl.Nullable:
			return refers(t.Elem, depth+1)
		case idl.Struct:
			for _, f := range t.Fields {
				if refers(f.Type, depth+1) {
					return true
				}
			}
		}
		return false
	}
	var out []string
	for _, mt := range iface.Methods {
		if refers(mt.In, 0) || refers(mt.Out, 0) {
			out = append(out, "method "+mt.Name)
		}
	}
	for _, td := range iface.Types {
		if td.Name != name && refers(td.Type, 0) {
			out = append(out, "type "+td.Name)
		}
	}
	for _, e := range iface.Errors {
		if refers(e.Params, 0) {
			out = append(out, "error "+e.Name)
		}
	}
	sort.Strings(out)
	return out
}

// errorDoc finds the IDL documentation for a qualified error name among the
// service's interfaces.
func errorDoc(svc *client.Service, name string) string {
	if svc == nil {
		return ""
	}
	i := strings.LastIndexByte(name, '.')
	if i <= 0 {
		return ""
	}
	iface := svc.Interface(name[:i])
	if iface == nil || iface.Parsed == nil {
		return ""
	}
	if e := iface.Parsed.Error(name[i+1:]); e != nil {
		return e.Doc
	}
	return ""
}
