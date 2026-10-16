package tui

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/sanitize"
)

type rowKind int

const (
	rowIface rowKind = iota
	rowMethod
	rowType
	rowError
)

// row is one line of the interfaces tree.
type row struct {
	kind    rowKind
	iface   *client.Interface
	method  *idl.Method
	typ     *idl.TypeDef
	err     *idl.Error
	verdict safety.Verdict
}

func (r row) name() string {
	switch r.kind {
	case rowMethod:
		return r.method.Name
	case rowType:
		return r.typ.Name
	case rowError:
		return r.err.Name
	}
	return r.iface.Name
}

func (r row) qualified() string {
	if r.kind == rowIface {
		return r.iface.Name
	}
	return r.iface.Name + "." + r.name()
}

func (m *Model) visibleSockets() []int {
	q := strings.ToLower(m.svcFilter.Value())
	var out []int
	for i, s := range m.sockets {
		if q == "" || strings.Contains(strings.ToLower(s.Name), q) || strings.Contains(strings.ToLower(s.Path), q) {
			out = append(out, i)
		}
	}
	return out
}

func (m *Model) selectedSocket() (discovery.Socket, bool) {
	vis := m.visibleSockets()
	if m.svcCursor < 0 || m.svcCursor >= len(vis) {
		return discovery.Socket{}, false
	}
	return m.sockets[vis[m.svcCursor]], true
}

func (m *Model) selectedRow() (row, bool) {
	if m.memCursor < 0 || m.memCursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.memCursor], true
}

// rebuildRows recomputes the interfaces tree from the open service, its
// expansion state and the filter. A filter shows every match regardless of
// expansion.
func (m *Model) rebuildRows() {
	m.rows = m.rows[:0]
	if m.svc == nil {
		return
	}
	q := strings.ToLower(m.memFilter.Value())
	match := func(s string) bool { return q == "" || strings.Contains(strings.ToLower(s), q) }
	for _, iface := range m.svc.Interfaces {
		var children []row
		if p := iface.Parsed; p != nil {
			for _, mt := range p.Methods {
				r := row{kind: rowMethod, iface: iface, method: mt}
				r.verdict = m.classifier.Classify(r.qualified())
				children = append(children, r)
			}
			for _, t := range p.Types {
				children = append(children, row{kind: rowType, iface: iface, typ: t})
			}
			for _, e := range p.Errors {
				children = append(children, row{kind: rowError, iface: iface, err: e})
			}
		}
		if q != "" {
			ifaceMatches := match(iface.Name)
			var kept []row
			for _, c := range children {
				if ifaceMatches || match(c.name()) {
					kept = append(kept, c)
				}
			}
			if len(kept) == 0 && !ifaceMatches {
				continue
			}
			m.rows = append(m.rows, row{kind: rowIface, iface: iface})
			m.rows = append(m.rows, kept...)
			continue
		}
		m.rows = append(m.rows, row{kind: rowIface, iface: iface})
		if m.expanded[iface.Name] {
			m.rows = append(m.rows, children...)
		}
	}
	if m.memCursor >= len(m.rows) {
		m.memCursor = max(len(m.rows)-1, 0)
	}
}

func (m *Model) browseHelpKeys() []key.Binding {
	if m.palette != nil {
		return []key.Binding{
			key.NewBinding(key.WithKeys("up"), key.WithHelp("↑↓", "choose")),
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "go")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "close")),
		}
	}
	if m.filtering {
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "apply")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "clear")),
		}
	}
	keys := []key.Binding{bk.Palette, bk.Up, bk.Down, bk.Next}
	switch m.focus {
	case paneServices:
		keys = append(keys, key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open")))
	case paneMembers:
		if r, ok := m.selectedRow(); ok && r.kind == rowMethod {
			keys = append(keys, bk.Call)
		} else {
			keys = append(keys, key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "expand")))
		}
		keys = append(keys, bk.IDL, bk.Copy)
	case paneDetail:
		keys = append(keys, bk.IDL)
	}
	return append(keys, bk.Filter, bk.Refresh, bk.Help, bk.Quit)
}

func (m *Model) updateBrowse(msg tea.KeyMsg) tea.Cmd {
	if m.palette != nil {
		return m.updatePalette(msg)
	}
	if m.filtering {
		return m.updateFilter(msg)
	}
	switch {
	case key.Matches(msg, bk.Quit):
		return tea.Quit
	case key.Matches(msg, bk.Palette):
		return m.openPalette()
	case key.Matches(msg, bk.Help):
		m.helpReturn = m.screen
		m.screen = screenHelp
		return nil
	case key.Matches(msg, bk.Back):
		switch {
		case m.showIDL:
			m.showIDL = false
		case m.focus == paneDetail:
			m.focus = paneMembers
		case m.focus == paneMembers && m.memFilter.Value() != "":
			m.memFilter.SetValue("")
			m.rebuildRows()
		case m.focus == paneMembers:
			m.focus = paneServices
		case m.svcFilter.Value() != "":
			m.svcFilter.SetValue("")
		}
		m.layout()
		return nil
	case m.focus == paneServices && (key.Matches(msg, bk.Next) || key.Matches(msg, bk.Right)):
		if m.svc == nil {
			return m.openSelected()
		}
		m.cycleFocus(1)
		return nil
	case key.Matches(msg, bk.Next), key.Matches(msg, bk.Right):
		m.cycleFocus(1)
		return nil
	case key.Matches(msg, bk.Prev), key.Matches(msg, bk.Left):
		m.cycleFocus(-1)
		return nil
	case key.Matches(msg, bk.Filter):
		if m.focus == paneDetail {
			return nil
		}
		m.filtering = true
		return m.filterInput().Focus()
	case key.Matches(msg, bk.Refresh):
		cmds := []tea.Cmd{discoverCmd(m.opts.Discovery)}
		if m.svcPath != "" {
			delete(m.cache, m.svcPath)
			cmds = append(cmds, m.introspect(m.svcPath))
		}
		return tea.Batch(append(cmds, m.setFlash("Refreshing…", true))...)
	case key.Matches(msg, bk.IDL):
		if m.svc != nil {
			m.showIDL = !m.showIDL
			m.refreshDetail()
		}
		return nil
	}

	switch m.focus {
	case paneServices:
		return m.updateServices(msg)
	case paneMembers:
		return m.updateMembers(msg)
	default:
		var cmd tea.Cmd
		m.detail, cmd = m.detail.Update(msg)
		return cmd
	}
}

func (m *Model) filterInput() *textinputModel {
	if m.focus == paneServices {
		return &m.svcFilter
	}
	return &m.memFilter
}

func (m *Model) updateFilter(msg tea.KeyMsg) tea.Cmd {
	in := m.filterInput()
	switch msg.String() {
	case "enter":
		m.filtering = false
		in.Blur()
		return nil
	case "esc":
		m.filtering = false
		in.Blur()
		in.SetValue("")
		return m.afterFilter()
	case "up", "down":
		m.filtering = false
		in.Blur()
		return m.updateBrowse(msg)
	}
	var cmd tea.Cmd
	*in, cmd = in.Update(msg)
	return tea.Batch(cmd, m.afterFilter())
}

func (m *Model) afterFilter() tea.Cmd {
	if m.focus == paneServices {
		m.svcCursor, m.svcOffset = 0, 0
		if s, ok := m.selectedSocket(); ok {
			return m.showService(s.Path)
		}
		m.refreshDetail()
		return nil
	}
	m.rebuildRows()
	m.memCursor, m.memOffset = 0, 0
	for i, r := range m.rows {
		if r.kind != rowIface {
			m.memCursor = i
			break
		}
	}
	m.refreshDetail()
	return nil
}

func (m *Model) cycleFocus(dir int) {
	order := []pane{paneServices, paneMembers, paneDetail}
	if m.svc == nil {
		order = []pane{paneServices, paneDetail}
	}
	idx := 0
	for i, p := range order {
		if p == m.focus {
			idx = i
		}
	}
	m.focus = order[(idx+dir+len(order))%len(order)]
	m.layout()
}

func moveCursor(cursor, n int, msg tea.KeyMsg, page int) int {
	switch {
	case key.Matches(msg, bk.Up):
		cursor--
	case key.Matches(msg, bk.Down):
		cursor++
	case key.Matches(msg, bk.PageUp):
		cursor -= page
	case key.Matches(msg, bk.PageDown):
		cursor += page
	case key.Matches(msg, bk.Top):
		cursor = 0
	case key.Matches(msg, bk.Bottom):
		cursor = n - 1
	default:
		return cursor
	}
	return clamp(cursor, 0, max(n-1, 0))
}

func (m *Model) updateServices(msg tea.KeyMsg) tea.Cmd {
	vis := m.visibleSockets()
	if key.Matches(msg, bk.Open) {
		return m.openSelected()
	}
	before := m.svcCursor
	m.svcCursor = moveCursor(m.svcCursor, len(vis), msg, m.bodyHeight()-3)
	if before != m.svcCursor {
		if s, ok := m.selectedSocket(); ok {
			return m.showService(s.Path)
		}
	}
	return nil
}

func (m *Model) updateMembers(msg tea.KeyMsg) tea.Cmd {
	r, ok := m.selectedRow()
	switch {
	case !ok:
		return nil
	case key.Matches(msg, bk.Open), key.Matches(msg, bk.Toggle):
		switch r.kind {
		case rowIface:
			if m.memFilter.Value() == "" {
				m.expanded[r.iface.Name] = !m.expanded[r.iface.Name]
				m.rebuildRows()
				m.refreshDetail()
			}
		case rowMethod:
			if key.Matches(msg, bk.Open) {
				return m.openCall(r)
			}
		default:
			m.focus = paneDetail
			m.layout()
		}
		return nil
	case key.Matches(msg, bk.Call):
		if r.kind == rowMethod {
			return m.openCall(r)
		}
		return nil
	case key.Matches(msg, bk.Copy):
		if r.kind == rowMethod {
			skel, _ := skeletonFor(r)
			params, _ := client.Compact([]byte(skel))
			vlx, _ := render.CallCommands(m.svcPath, r.qualified(), string(params), r.method.More == idl.MoreRequired, false)
			return copyCmd(vlx, "the vlx command")
		}
		return copyCmd(r.qualified(), "the name")
	}
	before := m.memCursor
	m.memCursor = moveCursor(m.memCursor, len(m.rows), msg, m.bodyHeight()-3)
	if before != m.memCursor {
		m.refreshDetail()
	}
	return nil
}

// viewBrowse renders the three panes.
func (m *Model) viewBrowse() string {
	sw, mw, dw := m.paneWidths()
	h := m.bodyHeight()
	var panes []string
	if sw > 0 {
		panes = append(panes, m.viewServices(sw, h))
	}
	if mw > 0 {
		panes = append(panes, m.viewMembers(mw, h))
	}
	if dw > 0 {
		panes = append(panes, m.viewDetail(dw, h))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, panes...)
}

// box draws a bordered pane of outer size w×h with a title and content
// lines.
func box(title string, focused bool, w, h int, lines []string) string {
	st, tst := paneStyle, paneTitleStyle
	if focused {
		st, tst = paneFocusedStyle, paneTitleFocusedStyle
	}
	inner := max(w-4, 1)
	content := []string{ansi.Truncate(tst.Render(title), inner, "…")}
	content = append(content, lines...)
	for len(content) < h-2 {
		content = append(content, "")
	}
	content = content[:max(h-2, 0)]
	return st.Width(w - 2).Height(h - 2).Render(strings.Join(content, "\n"))
}

// listWindow returns the slice bounds that keep cursor visible in height
// rows, updating offset.
func listWindow(cursor, n, height int, offset *int) (int, int) {
	if height <= 0 {
		return 0, 0
	}
	if cursor < *offset {
		*offset = cursor
	}
	if cursor >= *offset+height {
		*offset = cursor - height + 1
	}
	*offset = clamp(*offset, 0, max(n-height, 0))
	return *offset, min(*offset+height, n)
}

func renderItem(text string, width int, selected, focused bool) string {
	text = ansi.Truncate(text, width, "…")
	if !selected {
		return text
	}
	st := selectedStyle
	if focused {
		st = selectedFocusedStyle
	}
	return st.Width(width).Render(ansi.Strip(text))
}

func (m *Model) viewServices(w, h int) string {
	inner := w - 4
	focused := m.focus == paneServices
	var lines []string
	if m.filtering && focused || m.svcFilter.Value() != "" {
		m.svcFilter.Width = inner - 3
		lines = append(lines, m.svcFilter.View())
	}
	vis := m.visibleSockets()
	listH := h - 3 - len(lines)
	start, end := listWindow(m.svcCursor, len(vis), listH, &m.svcOffset)
	for i := start; i < end; i++ {
		s := m.sockets[vis[i]]
		dot := faintStyle.Render("○")
		if st := m.state[s.Path]; st != nil {
			switch {
			case st.loading:
				dot = m.spinner.View()
			case st.ok:
				dot = okStyle.Render("●")
			case st.err != nil:
				dot = errStyle.Render("✗")
			}
		}
		name := sanitize.Line(discovery.ShortName(s.Name))
		label := textStyle.Render(name)
		if !s.Accessible {
			label = mutedStyle.Render(name) + faintStyle.Render(" ⊘")
		}
		if s.Path == m.svcPath {
			label = boldStyle.Render(name)
		}
		lines = append(lines, renderItem(dot+" "+label, inner, i == m.svcCursor, focused))
	}
	if len(vis) == 0 {
		if !m.discovered {
			lines = append(lines, m.spinner.View()+mutedStyle.Render(" searching…"))
		} else if m.svcFilter.Value() != "" {
			lines = append(lines, mutedStyle.Render("no match"))
		} else {
			lines = append(lines, mutedStyle.Render("no sockets found"))
		}
	}
	title := "Services"
	if len(m.sockets) > 0 {
		title += mutedStyle.Render(" " + strconv.Itoa(len(vis)))
	}
	return box(title, focused, w, h, lines)
}

func (m *Model) viewMembers(w, h int) string {
	inner := w - 4
	focused := m.focus == paneMembers
	var lines []string
	if m.filtering && focused || m.memFilter.Value() != "" {
		m.memFilter.Width = inner - 3
		lines = append(lines, m.memFilter.View())
	}
	switch {
	case m.svc == nil && m.svcPath != "" && m.state[m.svcPath] != nil && m.state[m.svcPath].loading:
		lines = append(lines, m.spinner.View()+mutedStyle.Render(" introspecting…"))
	case m.svc == nil:
		lines = append(lines, mutedStyle.Render("Open a service to see"), mutedStyle.Render("its interfaces."))
	case len(m.rows) == 0:
		lines = append(lines, mutedStyle.Render("no match"))
	}
	listH := h - 3 - len(lines)
	start, end := listWindow(m.memCursor, len(m.rows), listH, &m.memOffset)
	for i := start; i < end; i++ {
		lines = append(lines, renderItem(m.rowLabel(m.rows[i], inner), inner, i == m.memCursor, focused))
	}
	title := "Interfaces"
	if m.svc != nil {
		title += mutedStyle.Render(" " + strconv.Itoa(len(m.svc.Interfaces)))
	}
	return box(title, focused, w, h, lines)
}

func (m *Model) rowLabel(r row, width int) string {
	switch r.kind {
	case rowIface:
		arrow := "▸"
		if m.expanded[r.iface.Name] || m.memFilter.Value() != "" {
			arrow = "▾"
		}
		label := brandStyle.Render(arrow) + " " + boldStyle.Render(sanitize.Line(r.iface.Name))
		if r.iface.Err != nil {
			label += " " + errStyle.Render("✗")
		}
		return label
	case rowMethod:
		glyph := brandStyle.Render("ƒ")
		if r.method.More != idl.MoreNone {
			glyph = brandStyle.Render("⌁")
		}
		tag := r.verdict.Class.String()
		name := sanitize.Line(r.method.Name)
		gap := width - 4 - ansi.StringWidth(name) - len(tag)
		if gap < 1 {
			name = ansi.Truncate(name, max(width-6-len(tag), 1), "…")
			gap = 1
		}
		return "  " + glyph + " " + textStyle.Render(name) + strings.Repeat(" ", gap) + classTag(r.verdict.Class)
	case rowType:
		return "  " + lipgloss.NewStyle().Foreground(render.Cyan).Render("τ") + " " + textStyle.Render(sanitize.Line(r.typ.Name))
	default:
		return "  " + errStyle.Render("!") + " " + textStyle.Render(sanitize.Line(r.err.Name))
	}
}
