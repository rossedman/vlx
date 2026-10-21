package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/cmdline"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/sanitize"
)

// command is the command line along the bottom of every page.
type command struct {
	input   textinput.Model
	focused bool
	err     string

	sugg   []entry
	cursor int
	moved  bool // a suggestion was picked with ↑↓

	// target is the method the line names, when it names one exactly.
	target *target

	pendingPath string // being introspected so that pendingLine can run
	pendingLine string
	indexed     bool

	recall    []string
	recallPos int
	draft     string
}

type target struct {
	path   string
	iface  *client.Interface
	method *idl.Method
	args   []cmdline.Arg
}

// newCommand builds an empty, unfocused command line.
func newCommand() command {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = brandStyle
	ti.Placeholder = "type to jump anywhere, or run a call like: resolve ResolveHostname name=example.com"
	ti.PlaceholderStyle = faintStyle
	ti.CharLimit = 4096
	staticCursorInTests(&ti.Cursor)
	return command{input: ti}
}

// focus gives the line the keyboard and resets ↑ history to the newest line.
func (c *command) focus() tea.Cmd {
	c.focused = true
	c.recallPos = len(c.recall)
	c.input.CursorEnd()
	return c.input.Focus()
}

// blur takes the keyboard away and drops the suggestions, target and error,
// keeping the text.
func (c *command) blur() {
	c.focused = false
	c.input.Blur()
	c.sugg, c.target, c.err, c.moved = nil, nil, "", false
}

// clear empties the line along with its suggestions, target and error.
func (c *command) clear() {
	c.input.SetValue("")
	c.sugg, c.target, c.err, c.moved, c.cursor = nil, nil, "", false, 0
}

// remember adds a line to the ↑ history.
func (c *command) remember(line string) {
	if n := len(c.recall); n > 0 && c.recall[n-1] == line {
		return
	}
	c.recall = append(c.recall, line)
	if len(c.recall) > 100 {
		c.recall = c.recall[1:]
	}
	c.recallPos = len(c.recall)
}

// updateCommand handles a key while the command line has the keyboard: esc,
// ⏎, ↑↓ through suggestions or history, ⇥ to complete, or an edit to the text.
func (m *Model) updateCommand(msg tea.KeyMsg) tea.Cmd {
	c := &m.cmd
	p := m.page()
	switch msg.String() {
	case "esc":
		if p.kind == pageMethod {
			c.blur()
			m.syncCommand()
			p.focus = focusForm
			return p.call.focusEditor()
		}
		c.clear()
		c.blur()
		return nil
	case "enter", "ctrl+s":
		return m.runLine(c.input.Value(), true)
	case "up", "ctrl+p":
		if len(c.sugg) > 0 {
			c.cursor = max(c.cursor-1, 0)
			c.moved = true
			return nil
		}
		return m.recallLine(-1)
	case "down", "ctrl+n":
		if len(c.sugg) > 0 {
			c.cursor = min(c.cursor+1, len(c.sugg)-1)
			c.moved = true
			return nil
		}
		return m.recallLine(1)
	case "tab":
		if len(c.sugg) > 0 {
			m.complete(c.sugg[c.cursor])
			return nil
		}
		if p.kind == pageMethod {
			c.blur()
			m.syncCommand()
			p.focus = focusResult
		}
		return nil
	case "shift+tab":
		if p.kind == pageMethod {
			c.blur()
			m.syncCommand()
			p.focus = focusForm
			return p.call.focusEditor()
		}
		return nil
	}
	before := c.input.Value()
	var cmd tea.Cmd
	c.input, cmd = c.input.Update(msg)
	if c.input.Value() != before {
		return tea.Batch(cmd, m.commandChanged())
	}
	return cmd
}

// recallLine steps through the lines of earlier calls.
func (m *Model) recallLine(d int) tea.Cmd {
	c := &m.cmd
	if len(c.recall) == 0 {
		return nil
	}
	if c.recallPos == len(c.recall) {
		c.draft = c.input.Value()
	}
	c.recallPos = clamp(c.recallPos+d, 0, len(c.recall))
	if c.recallPos == len(c.recall) {
		c.input.SetValue(c.draft)
	} else {
		c.input.SetValue(c.recall[c.recallPos])
	}
	c.input.CursorEnd()
	return m.commandChanged()
}

// complete fills in the line from a suggestion.
func (m *Model) complete(e entry) {
	line := m.svcToken(e.path) + " "
	if e.kind == entryMethod {
		line += e.name + " "
	}
	m.cmd.input.SetValue(line)
	m.cmd.input.CursorEnd()
	m.commandChanged()
}

// commandChanged reacts to an edit of the line: on a method page, a line
// for the same method updates the form; elsewhere it updates the
// suggestions.
func (m *Model) commandChanged() tea.Cmd {
	c := &m.cmd
	c.err, c.moved, c.cursor = "", false, 0
	var cmd tea.Cmd
	if !c.indexed {
		c.indexed = true
		cmd = m.indexAll()
	}
	m.refreshSuggestions()
	if mc := m.currentCall(); mc != nil && c.target != nil && c.target.method == mc.method && c.target.path == mc.path {
		if err := mc.setArgs(c.target.args); err != nil {
			c.err = err.Error()
		}
		m.refreshResponse(mc, false)
	}
	return cmd
}

// refreshSuggestions recomputes what the line matches.
func (m *Model) refreshSuggestions() {
	c := &m.cmd
	c.sugg, c.target = nil, nil
	value := c.input.Value()
	if !c.focused || strings.TrimSpace(value) == "" {
		return
	}
	l, err := cmdline.Parse(value)
	if err != nil {
		if !errors.Is(err, cmdline.ErrUnterminated) {
			c.err = err.Error()
		}
		return
	}
	if path, iface, mt, err := m.resolveLine(l); err == nil && mt != nil {
		c.target = &target{path: path, iface: iface, method: mt, args: l.Args}
		return
	}
	if len(l.Args) > 0 {
		return
	}
	c.sugg = m.search(strings.TrimSpace(l.Service+" "+l.Method), 50)
	c.cursor = clamp(c.cursor, 0, max(len(c.sugg)-1, 0))
}

var errNotLoaded = errors.New("not introspected yet")

// resolveLine finds what a parsed line names. It returns errNotLoaded
// with the path when the service has not been introspected.
func (m *Model) resolveLine(l cmdline.Line) (string, *client.Interface, *idl.Method, error) {
	path, err := discovery.Resolve(l.Service, m.sockets)
	var only *client.Interface
	if err != nil {
		if l.Method == "" {
			if path, iface, mt, ok := m.findMethod(l.Service); ok {
				return path, iface, mt, nil
			}
		}
		var ok bool
		if path, only, ok = m.findInterface(l.Service); !ok {
			return "", nil, nil, err
		}
	}
	svc := m.cache[path]
	if svc == nil {
		return path, nil, nil, errNotLoaded
	}
	if l.Method == "" {
		return path, nil, nil, nil
	}
	if only != nil {
		svc = &client.Service{Path: path, Interfaces: []*client.Interface{only}}
	}
	if iface, mt := methodIn(svc, l.Method); mt != nil {
		return path, iface, mt, nil
	}
	return path, nil, nil, m.noMethod(path, svc, l.Method)
}

// findInterface looks a word up as an interface name, whole or its last
// part ("unit" for io.systemd.Unit), in every introspected service; it
// succeeds when exactly one service has it.
func (m *Model) findInterface(word string) (string, *client.Interface, bool) {
	var (
		path  string
		found *client.Interface
		n     int
	)
	for _, s := range m.sockets {
		svc := m.cache[s.Path]
		if svc == nil {
			continue
		}
		for _, i := range svc.Interfaces {
			short := i.Name[strings.LastIndexByte(i.Name, '.')+1:]
			if !boilerplate(i.Name) && (i.Name == word || strings.EqualFold(short, word)) {
				path, found = s.Path, i
				n++
			}
		}
	}
	return path, found, n == 1
}

// methodIn finds a method of svc by name, qualified name or, failing
// those, case-insensitive name.
func methodIn(svc *client.Service, name string) (*client.Interface, *idl.Method) {
	for pass := 0; pass < 2; pass++ {
		for _, iface := range svc.Interfaces {
			if iface.Parsed == nil {
				continue
			}
			for _, mt := range iface.Parsed.Methods {
				q := iface.Name + "." + mt.Name
				if pass == 0 && (mt.Name == name || q == name) || pass == 1 && strings.EqualFold(mt.Name, name) {
					return iface, mt
				}
			}
		}
	}
	return nil, nil
}

// findMethod looks a bare or qualified method name up in every
// introspected service; it succeeds when exactly one has it.
func (m *Model) findMethod(name string) (string, *client.Interface, *idl.Method, bool) {
	var (
		path  string
		iface *client.Interface
		mt    *idl.Method
		n     int
	)
	for _, s := range m.sockets {
		svc := m.cache[s.Path]
		if svc == nil {
			continue
		}
		for _, i := range svc.Interfaces {
			if i.Parsed == nil || boilerplate(i.Name) {
				continue
			}
			for _, x := range i.Parsed.Methods {
				if x.Name == name || i.Name+"."+x.Name == name {
					path, iface, mt = s.Path, i, x
					n++
				}
			}
		}
	}
	return path, iface, mt, n == 1
}

// noMethod is the error for a line naming a method the service lacks; it
// lists up to eight of the methods the service does have.
func (m *Model) noMethod(path string, svc *client.Service, name string) error {
	var names []string
	for _, iface := range svc.Interfaces {
		if iface.Parsed == nil || boilerplate(iface.Name) {
			continue
		}
		for _, mt := range iface.Parsed.Methods {
			names = append(names, mt.Name)
		}
	}
	s, _ := socketFor(path, m.sockets)
	msg := fmt.Sprintf("%s has no method %q", label(s), name)
	if len(names) > 0 {
		if len(names) > 8 {
			names = append(names[:8], "…")
		}
		msg += " (methods: " + strings.Join(names, ", ") + ")"
	}
	return errors.New(msg)
}

// runLine does what a command line says: open a service, open a method,
// or fill in and send a call. fromCmd is true when the user pressed ⏎ on
// the line, so suggestions may stand in for a line that names nothing.
func (m *Model) runLine(line string, fromCmd bool) tea.Cmd {
	c := &m.cmd
	l, err := cmdline.Parse(line)
	if err != nil {
		c.err = err.Error()
		return m.setFlash(err.Error(), false)
	}
	if l.Service == "" {
		return nil
	}
	if fromCmd && c.moved && c.cursor < len(c.sugg) {
		return m.jumpTo(c.sugg[c.cursor])
	}
	path, iface, mt, err := m.resolveLine(l)
	switch {
	case errors.Is(err, errNotLoaded):
		if st := m.state[path]; st != nil && st.err != nil && !st.loading {
			delete(m.state, path) // the user asked again: try again
		}
		c.pendingPath, c.pendingLine = path, line
		return m.introspect(path)
	case err != nil:
		if fromCmd && path == "" && len(l.Args) == 0 && len(c.sugg) > 0 {
			return m.jumpTo(c.sugg[0])
		}
		c.err = err.Error()
		return m.setFlash(err.Error(), false)
	case mt == nil:
		c.blur()
		return m.openService(path, "")
	}

	cur := m.currentCall()
	already := cur != nil && cur.path == path && cur.method == mt
	var cmds []tea.Cmd
	if !already {
		cmds = append(cmds, m.openMethod(path, iface, mt))
	}
	call := m.currentCall()
	if err := call.setArgs(l.Args); err != nil {
		c.err = err.Error()
		m.refreshResponse(call, false)
		return tea.Batch(append(cmds, m.setFlash(err.Error(), false))...)
	}
	m.refreshResponse(call, false)
	send := already || len(l.Args) > 0 || !call.form.required()
	if !send {
		c.blur()
		m.syncCommand()
		return tea.Batch(cmds...)
	}
	if fromCmd {
		call.blurEditor()
		cmds = append(cmds, c.focus())
	}
	cmds = append(cmds, m.trySend(call))
	if fromCmd {
		c.input.SetValue(sanitize.Line(m.canonical(call)))
		c.input.CursorEnd()
		m.refreshSuggestions()
	}
	return tea.Batch(cmds...)
}

// jumpTo opens a search result.
func (m *Model) jumpTo(e entry) tea.Cmd {
	m.cmd.clear()
	m.cmd.blur()
	if e.kind == entryService {
		return m.openService(e.path, "")
	}
	svc := m.cache[e.path]
	if svc == nil {
		return m.openService(e.path, "")
	}
	iface := svc.Interface(e.iface)
	if iface == nil || iface.Parsed == nil {
		return m.openService(e.path, e.iface)
	}
	switch e.kind {
	case entryMethod:
		if mt := iface.Parsed.Method(e.name); mt != nil {
			return m.openMethod(e.path, iface, mt)
		}
	case entryType:
		if t := iface.Parsed.Type(e.name); t != nil {
			cmd := m.openService(e.path, e.iface)
			m.page().sel = "t:" + e.iface + "." + e.name
			m.openDetail(e.path, item{kind: itemType, iface: iface, typ: t}, false)
			return cmd
		}
	case entryError:
		if er := iface.Parsed.Error(e.name); er != nil {
			cmd := m.openService(e.path, e.iface)
			m.page().sel = "e:" + e.iface + "." + e.name
			m.openDetail(e.path, item{kind: itemError, iface: iface, err: er}, false)
			return cmd
		}
	}
	return m.openService(e.path, e.iface)
}

// canonical is the command line for the call on a method page.
func (m *Model) canonical(c *callState) string {
	args, err := c.args()
	if err != nil {
		return m.callLine(c, nil)
	}
	return m.callLine(c, args)
}

// syncCommand shows the method page's call on the command line, unless
// the user is editing the line.
func (m *Model) syncCommand() {
	c := m.currentCall()
	if c == nil || m.cmd.focused {
		return
	}
	m.cmd.input.SetValue(sanitize.Line(m.canonical(c)))
	m.cmd.input.CursorEnd()
}

// viewCommand renders the command line, noting on a method page that it
// mirrors the form.
func (m *Model) viewCommand() string {
	in := m.cmd.input
	in.Width = max(m.width-2*gutter-4, 10)
	line := strings.Repeat(" ", gutter) + in.View()
	if !m.cmd.focused && m.page().kind == pageMethod {
		note := faintStyle.Render("mirrors the form · ⇥ to edit here ")
		if w := m.width - ansi.StringWidth(line) - ansi.StringWidth(note); w > 2 {
			line += strings.Repeat(" ", w) + note
		}
	}
	return ansi.Truncate(line, m.width, "…")
}

const suggestionRows = 6

// viewSuggestions renders the popup above the command line, or "".
func (m *Model) viewSuggestions(width int) string {
	c := &m.cmd
	if !c.focused {
		return ""
	}
	w := min(width, 100)
	inner := w - 4
	var lines []string
	switch {
	case c.target != nil:
		if mc := m.currentCall(); mc != nil && mc.method == c.target.method && mc.path == c.target.path {
			return ""
		}
		lines = m.paramHints(c.target, inner)
	case len(c.sugg) > 0:
		q := strings.TrimSpace(c.input.Value())
		head := mutedStyle.Render("matches for ") + textStyle.Render(sanitize.Line(q)) + mutedStyle.Render(" — ⏎ opens · ⇥ completes · keep typing to call")
		lines = append(lines, ansi.Truncate(head, inner, "…"))
		start, end := listWindow(c.cursor, len(c.sugg), suggestionRows, new(int))
		for i := start; i < end; i++ {
			lines = append(lines, m.entryRow(c.sugg[i], inner, i == c.cursor))
		}
	case strings.TrimSpace(c.input.Value()) != "":
		lines = append(lines, mutedStyle.Render("No matches. ")+faintStyle.Render("Try a service name, a method name, or a socket path."))
	default:
		return ""
	}
	if n := m.indexing(); n > 0 {
		lines = append(lines, m.spinner.View()+mutedStyle.Render(fmt.Sprintf(" reading %d more %s…", n, plural(n, "service", "services"))))
	} else if !m.cfg.Preview {
		lines = append(lines, faintStyle.Render("preview is off: only services you have opened are searched"))
	}
	return dialogStyle.Padding(0, 1).BorderForeground(render.BrandDeep).Width(w - 2).Render(strings.Join(lines, "\n"))
}

// paramHints lists the parameters of the method a line names.
func (m *Model) paramHints(t *target, width int) []string {
	given := map[string]string{}
	for _, a := range t.args {
		given[a.Name] = a.Value
	}
	v := m.classifier.Classify(t.iface.Name + "." + t.method.Name)
	s, _ := socketFor(t.path, m.sockets)
	head := boldStyle.Render(sanitize.Line(label(s)+" › "+t.method.Name)) + "  " + classTag(v.Class)
	if doc := summary(t.method.Doc); doc != "" {
		head += mutedStyle.Render("  " + sanitize.Line(doc))
	}
	lines := []string{ansi.Truncate(head, width, "…")}
	var fields []*idl.Field
	if t.method.In != nil {
		fields = t.method.In.Fields
	}
	for i, f := range fields {
		if i == suggestionRows-1 && len(fields) > suggestionRows {
			lines = append(lines, faintStyle.Render(fmt.Sprintf("  … %d more", len(fields)-i)))
			break
		}
		val := faintStyle.Render("not set")
		if g, ok := given[f.Name]; ok {
			val = render.StringStyle.Render(sanitize.Line(g))
		} else if f.Type.Kind == idl.Nullable {
			val = faintStyle.Render("optional")
		}
		lines = append(lines, columns(width, "  "+render.KeyStyle.Render(sanitize.Line(f.Name))+"="+val, mutedStyle.Render(sanitize.Line(summary(f.Doc))), render.TypeStyle.Render(sanitize.Line(f.Type.String()))))
	}
	what := "⏎ opens the method page"
	required := false
	for _, f := range fields {
		required = required || f.Type.Kind != idl.Nullable
	}
	if len(t.args) > 0 || !required {
		what = "⏎ sends the call"
	}
	if len(fields) == 0 {
		lines = append(lines, mutedStyle.Render("  no parameters"))
	}
	return append(lines, faintStyle.Render(what+" · add name=value to set a parameter"))
}
