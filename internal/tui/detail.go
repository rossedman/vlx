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
	"github.com/rossedman/vlx/internal/schema"
)

func (m *Model) viewDetail(w, h int) string {
	title := "Detail"
	if m.showIDL {
		title = "IDL"
	}
	if m.detail.TotalLineCount() > m.detail.Height {
		title += mutedStyle.Render(fmt.Sprintf(" %d%%", int(m.detail.ScrollPercent()*100)))
	}
	return box(title, m.focus == paneDetail, w, h, strings.Split(m.detail.View(), "\n"))
}

// refreshDetail regenerates the detail pane for the current selection.
func (m *Model) refreshDetail() {
	width := max(m.detail.Width, 10)
	var content string
	switch {
	case m.focus == paneServices || m.svc == nil:
		content = m.detailService(width)
	default:
		r, ok := m.selectedRow()
		switch {
		case !ok:
			content = m.detailService(width)
		case m.showIDL:
			content = render.IDL(r.iface.Description, true)
			if r.iface.Err != nil && r.iface.Description == "" {
				content = errStyle.Render(sanitize.Text(r.iface.Err.Error()))
			}
		default:
			content = m.detailRow(r, width)
		}
	}
	m.detail.SetContent(content)
	m.detail.GotoTop()
}

func wrap(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

func heading(s string) string {
	return sectionStyle.Render(s)
}

var logo = []string{
	"██╗   ██╗██╗     ██╗  ██╗",
	"██║   ██║██║     ╚██╗██╔╝",
	"╚██╗ ██╔╝██║      ╚███╔╝ ",
	" ╚████╔╝ ██║      ██╔██╗ ",
	"  ╚██╔╝  ███████╗██╔╝ ██╗",
	"   ╚═╝   ╚══════╝╚═╝  ╚═╝",
}

var logoColors = []lipgloss.AdaptiveColor{
	{Light: "#7C3AED", Dark: "#C4B5FD"},
	{Light: "#7C3AED", Dark: "#A78BFA"},
	{Light: "#6D28D9", Dark: "#8B5CF6"},
	{Light: "#5B21B6", Dark: "#7C3AED"},
	{Light: "#0E7490", Dark: "#22D3EE"},
	{Light: "#0E7490", Dark: "#06B6D4"},
}

func (m *Model) welcome(width int) string {
	var b strings.Builder
	if width >= 28 {
		for i, l := range logo {
			b.WriteString(lipgloss.NewStyle().Foreground(logoColors[i]).Render(l))
			b.WriteByte('\n')
		}
		b.WriteString(mutedStyle.Render("Varlink explorer"))
		b.WriteString("\n\n")
	}
	switch {
	case !m.discovered:
		b.WriteString(mutedStyle.Render("Looking for Varlink sockets…"))
	case len(m.sockets) == 0:
		b.WriteString(wrap(textStyle.Render("No Varlink sockets were found in the usual places."), width))
		b.WriteString("\n\n")
		b.WriteString(wrap(mutedStyle.Render("systemd v257 or newer serves its APIs under /run/systemd. Try:"), width))
		b.WriteString("\n\n  " + brandStyle.Render("vlx demo") + mutedStyle.Render("          a sample service"))
		b.WriteString("\n  " + brandStyle.Render("vlx PATH") + mutedStyle.Render("          a socket anywhere"))
		b.WriteString("\n  " + brandStyle.Render("--search DIR") + mutedStyle.Render("      look somewhere else"))
	default:
		b.WriteString(textStyle.Render(fmt.Sprintf("Found %d services. ", len(m.sockets))))
		b.WriteString(mutedStyle.Render("Pick one and press ") + brandStyle.Render("⏎") + mutedStyle.Render("."))
		b.WriteString("\n\n")
		intro := "Nothing is sent to a service until you open it. "
		if m.cfg.Preview {
			intro = "As you move, vlx reads each service's description with read-only introspection. "
		}
		b.WriteString(wrap(mutedStyle.Render(intro+modeExplanation(m.policy.AllowWrites)), width))
	}
	return b.String()
}

func modeExplanation(writes bool) string {
	if writes {
		return "Writes are enabled: vlx asks before every WRITE call and makes you type the name of DANGER methods."
	}
	return "vlx is read-only: you can prepare any call, but only READ methods are sent. Restart with --allow-writes to change that."
}

func (m *Model) detailService(width int) string {
	s, ok := m.selectedSocket()
	if m.focus != paneServices && m.svcPath != "" {
		s, _ = socketFor(m.svcPath, m.sockets)
		ok = true
	}
	if !ok {
		return m.welcome(width)
	}
	var b strings.Builder
	b.WriteString(boldStyle.Render(sanitize.Line(s.Name)) + "\n")
	b.WriteString(mutedStyle.Render(sanitize.Line(s.Path)) + "\n")
	if !s.Accessible {
		b.WriteString("\n" + wrap(warnStyle.Render("⊘ You do not have permission to connect to this socket. Try sudo."), width) + "\n")
	}
	st := m.state[s.Path]
	switch {
	case st == nil:
		b.WriteString("\n" + mutedStyle.Render("Press ") + brandStyle.Render("⏎") + mutedStyle.Render(" to connect and introspect."))
		return b.String()
	case st.loading:
		b.WriteString("\n" + m.spinner.View() + mutedStyle.Render(" introspecting…"))
		return b.String()
	case st.err != nil:
		b.WriteString("\n" + errStyle.Render("✗ ") + wrap(errStyle.Render(sanitize.Line(st.err.Error())), width-2))
		if hint := client.Hint(st.err); hint != "" {
			b.WriteString("\n\n" + wrap(mutedStyle.Render(hint), width))
		}
		return b.String()
	}
	if m.svc == nil || m.svc.Path != s.Path {
		b.WriteString("\n" + mutedStyle.Render("Press ⏎ to open."))
		return b.String()
	}
	info := m.svc.Info
	if m.focus == paneServices {
		b.WriteString("\n" + mutedStyle.Render("Press ") + brandStyle.Render("⏎") + mutedStyle.Render(" or ") + brandStyle.Render("→") + mutedStyle.Render(" to explore its interfaces.") + "\n")
	}
	b.WriteString(heading("Service") + "\n")
	for _, kv := range [][2]string{
		{"vendor", info.Vendor}, {"product", info.Product}, {"version", info.Version}, {"url", info.URL},
	} {
		if kv[1] != "" {
			b.WriteString(fmt.Sprintf("%s %s\n", mutedStyle.Render(fmt.Sprintf("%-8s", kv[0])), textStyle.Render(sanitize.Line(kv[1]))))
		}
	}
	b.WriteString(heading("Interfaces") + "\n")
	for _, i := range m.svc.Interfaces {
		line := "• " + textStyle.Render(sanitize.Line(i.Name))
		if i.Parsed != nil {
			line += mutedStyle.Render(fmt.Sprintf("  %d methods", len(i.Parsed.Methods)))
		} else if i.Err != nil {
			line += errStyle.Render("  ✗ " + sanitize.Line(i.Err.Error()))
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func (m *Model) detailRow(r row, width int) string {
	var b strings.Builder
	switch r.kind {
	case rowIface:
		b.WriteString(boldStyle.Render(sanitize.Line(r.iface.Name)) + "\n")
		if r.iface.Parsed == nil {
			b.WriteString("\n" + errStyle.Render("✗ ") + wrap(errStyle.Render(sanitize.Line(fmt.Sprint(r.iface.Err))), width-2))
			return b.String()
		}
		p := r.iface.Parsed
		if p.Doc != "" {
			b.WriteString("\n" + wrap(docStyle.Render(sanitize.Text(p.Doc)), width) + "\n")
		}
		b.WriteString("\n" + mutedStyle.Render(fmt.Sprintf("%d methods · %d types · %d errors", len(p.Methods), len(p.Types), len(p.Errors))) + "\n")
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
		b.WriteString("\n" + mutedStyle.Render("Press i for the IDL source."))
	case rowMethod:
		m.detailMethod(&b, r, width)
	case rowType:
		b.WriteString(mutedStyle.Render("type ") + boldStyle.Render(sanitize.Line(r.typ.Name)) + "\n")
		b.WriteString(mutedStyle.Render(sanitize.Line(r.iface.Name)) + "\n")
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
	case rowError:
		b.WriteString(mutedStyle.Render("error ") + errStyle.Bold(true).Render(sanitize.Line(r.err.Name)) + "\n")
		b.WriteString(mutedStyle.Render(sanitize.Line(r.qualified())) + "\n")
		if r.err.Doc != "" {
			b.WriteString("\n" + wrap(docStyle.Render(sanitize.Text(r.err.Doc)), width) + "\n")
		}
		b.WriteString(heading("Parameters") + "\n")
		writeFields(&b, r.err.Params, width)
	}
	return b.String()
}

func kindLabel(t *idl.Type) string {
	if t != nil && t.Kind == idl.Enum {
		return "Values"
	}
	return "Fields"
}

func (m *Model) detailMethod(b *strings.Builder, r row, width int) {
	mt := r.method
	b.WriteString(mutedStyle.Render("method ") + boldStyle.Render(sanitize.Line(mt.Name)) + "\n")
	b.WriteString(mutedStyle.Render(sanitize.Line(r.iface.Name)) + "\n\n")

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
	b.WriteString("\n" + mutedStyle.Render("Press ") + brandStyle.Render("⏎") + mutedStyle.Render(" or ") + brandStyle.Render("c") + mutedStyle.Render(" to call, ") + brandStyle.Render("y") + mutedStyle.Render(" to copy the command."))
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

// skeletonFor returns the parameter skeleton for a method row.
func skeletonFor(r row) (string, []string) {
	return schema.Skeleton(r.iface.Parsed, r.method.In)
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
