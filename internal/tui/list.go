package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/cmdline"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
)

type itemKind int

const (
	itemHeading itemKind = iota // a group title; not selectable
	itemSocket
	itemHistory
	itemIface
	itemMethod
	itemType
	itemError
)

// item is one line of a list page.
type item struct {
	kind    itemKind
	heading string
	sock    discovery.Socket
	hist    int
	iface   *client.Interface
	method  *idl.Method
	typ     *idl.TypeDef
	err     *idl.Error
	verdict safety.Verdict
}

// key identifies the item across rebuilds of the list, for keeping the selection.
func (it item) key() string {
	switch it.kind {
	case itemSocket:
		return "s:" + it.sock.Path
	case itemHistory:
		return fmt.Sprintf("h:%d", it.hist)
	case itemIface:
		return "i:" + it.iface.Name
	case itemMethod:
		return "m:" + it.iface.Name + "." + it.method.Name
	case itemType:
		return "t:" + it.iface.Name + "." + it.typ.Name
	case itemError:
		return "e:" + it.iface.Name + "." + it.err.Name
	}
	return ""
}

// name is the item's short name, or the title of a heading.
func (it item) name() string {
	switch it.kind {
	case itemMethod:
		return it.method.Name
	case itemType:
		return it.typ.Name
	case itemError:
		return it.err.Name
	case itemIface:
		return it.iface.Name
	}
	return it.heading
}

// qualified is the item's name with its interface, as in io.systemd.Foo.Bar.
func (it item) qualified() string {
	if it.kind == itemIface {
		return it.iface.Name
	}
	return it.iface.Name + "." + it.name()
}

// items lists the lines of a list page.
func (m *Model) items(p *page) []item {
	if p.kind == pageHome {
		return m.homeItems()
	}
	return m.serviceItems(p)
}

// homeItems lists the three most recent calls, then the sockets grouped by kind.
func (m *Model) homeItems() []item {
	var out []item
	if n := len(m.history); n > 0 {
		out = append(out, item{kind: itemHeading, heading: "Recent calls"})
		for i := n - 1; i >= max(n-3, 0); i-- {
			out = append(out, item{kind: itemHistory, hist: i})
		}
	}
	groups := map[string][]discovery.Socket{}
	for _, s := range m.sockets {
		g, _ := describe(s)
		groups[g] = append(groups[g], s)
	}
	for _, g := range groupOrder {
		if len(groups[g]) == 0 {
			continue
		}
		out = append(out, item{kind: itemHeading, heading: g})
		for _, s := range groups[g] {
			out = append(out, item{kind: itemSocket, sock: s})
		}
	}
	return out
}

// serviceItems lists a service's interfaces, with the methods, types and errors
// of the expanded ones. It is empty until the service is introspected.
func (m *Model) serviceItems(p *page) []item {
	svc := m.cache[p.path]
	if svc == nil {
		return nil
	}
	var out []item
	for _, iface := range svc.Interfaces {
		out = append(out, item{kind: itemIface, iface: iface})
		if !p.isExpanded(iface.Name) || iface.Parsed == nil {
			continue
		}
		for _, mt := range iface.Parsed.Methods {
			it := item{kind: itemMethod, iface: iface, method: mt}
			it.verdict = m.classifier.Classify(it.qualified())
			out = append(out, it)
		}
		for _, t := range iface.Parsed.Types {
			out = append(out, item{kind: itemType, iface: iface, typ: t})
		}
		for _, e := range iface.Parsed.Errors {
			out = append(out, item{kind: itemError, iface: iface, err: e})
		}
	}
	return out
}

// cursorIndex finds the selected item, falling back to the first method
// or else the first selectable item.
func cursorIndex(items []item, sel string) int {
	first, firstMethod := -1, -1
	for i, it := range items {
		if it.kind == itemHeading {
			continue
		}
		if it.key() == sel {
			return i
		}
		if first < 0 {
			first = i
		}
		if firstMethod < 0 && it.kind == itemMethod {
			firstMethod = i
		}
	}
	if firstMethod >= 0 {
		return firstMethod
	}
	return first
}

// selected returns the item under the cursor, if the page has any.
func (m *Model) selected(p *page) (item, bool) {
	items := m.items(p)
	i := cursorIndex(items, p.sel)
	if i < 0 {
		return item{}, false
	}
	return items[i], true
}

// moveSel moves the selection by d selectable items, or to an end.
func moveSel(items []item, sel string, d int) string {
	var idx []int
	cur := 0
	ci := cursorIndex(items, sel)
	for i, it := range items {
		if it.kind == itemHeading {
			continue
		}
		if i == ci {
			cur = len(idx)
		}
		idx = append(idx, i)
	}
	if len(idx) == 0 {
		return sel
	}
	return items[idx[clamp(cur+d, 0, len(idx)-1)]].key()
}

// isTyping reports whether a key on a list page should start the command
// line instead.
func isTyping(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRunes && !msg.Alt && len(msg.Runes) > 0
}

// updateList handles a key on a list page. Typing a letter starts the command
// line; moving the selection on the home page previews the socket.
func (m *Model) updateList(p *page, msg tea.KeyMsg) tea.Cmd {
	items := m.items(p)
	page := max(m.bodyHeight()-4, 1)
	before := p.sel
	switch {
	case msg.String() == "q":
		m.stopAll()
		return tea.Quit
	case msg.String() == "?":
		return m.openHelp()
	case msg.String() == "/" || msg.String() == ":" || key.Matches(msg, gk.Command):
		return m.cmd.focus()
	case isTyping(msg):
		return tea.Batch(m.cmd.focus(), m.updateCommand(msg))
	case key.Matches(msg, lk.Up):
		p.sel = moveSel(items, p.sel, -1)
	case key.Matches(msg, lk.Down):
		p.sel = moveSel(items, p.sel, 1)
	case key.Matches(msg, lk.PageUp):
		p.sel = moveSel(items, p.sel, -page)
	case key.Matches(msg, lk.PageDown):
		p.sel = moveSel(items, p.sel, page)
	case key.Matches(msg, lk.Top):
		p.sel = moveSel(items, p.sel, -len(items))
	case key.Matches(msg, lk.Bottom):
		p.sel = moveSel(items, p.sel, len(items))
	case key.Matches(msg, lk.Back):
		return m.pop()
	case key.Matches(msg, lk.Refresh):
		return m.refresh(p)
	case key.Matches(msg, lk.Open), key.Matches(msg, lk.Toggle):
		return m.activate(p, msg)
	case key.Matches(msg, lk.Docs):
		if it, ok := m.selected(p); ok && p.kind == pageService {
			m.openDetail(p.path, it, it.kind == itemIface)
		}
		return nil
	case key.Matches(msg, lk.Copy):
		return m.copySelected(p)
	}
	if p.sel != before && p.kind == pageHome {
		return m.previewSelected()
	}
	return nil
}

// refresh rediscovers sockets and, on a service page, drops the cached
// introspection and reads it again.
func (m *Model) refresh(p *page) tea.Cmd {
	cmds := []tea.Cmd{discoverCmd(m.opts.Discovery), m.setFlash("Refreshing…", true)}
	if p.kind == pageService {
		delete(m.cache, p.path)
		delete(m.state, p.path)
		cmds = append(cmds, m.introspect(p.path))
	}
	return tea.Batch(cmds...)
}

// activate opens or toggles the selected item.
func (m *Model) activate(p *page, msg tea.KeyMsg) tea.Cmd {
	it, ok := m.selected(p)
	if !ok {
		return nil
	}
	toggle := key.Matches(msg, lk.Toggle)
	switch it.kind {
	case itemSocket:
		if !it.sock.Accessible {
			return m.setFlash("You do not have permission to connect to "+it.sock.Path+"; try sudo.", false)
		}
		return m.openService(it.sock.Path, "")
	case itemHistory:
		return m.reopen(m.history[it.hist])
	case itemIface:
		if msg.String() == "right" {
			m.openDetail(p.path, it, true)
			return nil
		}
		p.expanded[it.iface.Name] = !p.isExpanded(it.iface.Name)
		return nil
	case itemMethod:
		if toggle {
			return nil
		}
		return m.openMethod(p.path, it.iface, it.method)
	default:
		if !toggle {
			m.openDetail(p.path, it, false)
		}
	}
	return nil
}

// reopen shows the method page of an earlier call with its arguments,
// ready to send again.
func (m *Model) reopen(h historyEntry) tea.Cmd {
	svc := m.cache[h.path]
	if svc == nil {
		return m.openService(h.path, "")
	}
	iface, mt := methodIn(svc, h.qualified)
	if mt == nil {
		return m.openService(h.path, "")
	}
	cmd := m.openMethod(h.path, iface, mt)
	c := m.currentCall()
	if l, err := cmdline.Parse(h.line); err == nil && c.setArgs(l.Args) == nil {
		m.refreshResponse(c, false)
		m.syncCommand()
	}
	return cmd
}

// copySelected copies what is most useful for the selected item: a socket's path,
// a vlx command for a call or method, or else the qualified name.
func (m *Model) copySelected(p *page) tea.Cmd {
	it, ok := m.selected(p)
	if !ok {
		return nil
	}
	switch it.kind {
	case itemSocket:
		return copyCmd(it.sock.Path, "the socket path")
	case itemHistory:
		return copyCmd("vlx call "+m.history[it.hist].line, "the vlx command")
	case itemMethod:
		return copyCmd("vlx call "+m.svcToken(p.path)+" "+it.method.Name, "the vlx command")
	}
	return copyCmd(it.qualified(), "the name")
}

// renderList draws items in height lines, keeping the selection visible.
func (m *Model) renderList(p *page, items []item, width, height int, focused bool) []string {
	ci := cursorIndex(items, p.sel)
	if ci >= 0 {
		p.sel = items[ci].key()
	}
	var lines []string
	selLine := 0
	for i, it := range items {
		if it.kind == itemHeading && i > 0 {
			lines = append(lines, "")
		}
		if i == ci {
			selLine = len(lines)
		}
		lines = append(lines, m.renderItem(p, it, width, i == ci, focused))
	}
	if ci > 0 && items[ci-1].kind == itemHeading && selLine-1 < p.offset {
		p.offset = selLine - 1 // keep a group's title with its first item
	}
	start, end := listWindow(selLine, len(lines), height, &p.offset)
	return lines[start:end]
}

// renderItem draws one list line, highlighted when selected.
func (m *Model) renderItem(p *page, it item, width int, selected, focused bool) string {
	var line string
	switch it.kind {
	case itemHeading:
		return brandStyle.Render(strings.ToUpper(it.heading))
	case itemSocket:
		line = m.socketLine(it.sock, width)
	case itemHistory:
		line = m.historyLine(m.history[it.hist], width)
	case itemIface:
		arrow := "▸"
		if p.isExpanded(it.iface.Name) {
			arrow = "▾"
		}
		meta := ""
		if pi := it.iface.Parsed; pi != nil {
			meta = counts(pi)
		} else if it.iface.Err != nil {
			meta = errStyle.Render("✗ " + it.iface.Err.Error())
		}
		line = columns(width, brandStyle.Render(arrow)+" "+boldStyle.Render(it.iface.Name), "", mutedStyle.Render(meta))
	case itemMethod:
		glyph := "ƒ"
		if it.method.More != idl.MoreNone {
			glyph = "⌁"
		}
		line = columns(width, "  "+brandStyle.Render(glyph)+" "+textStyle.Render(it.method.Name), mutedStyle.Render(summary(it.method.Doc)), classTag(it.verdict.Class))
	case itemType:
		line = columns(width, "  "+typeGlyph+" "+textStyle.Render(it.typ.Name), mutedStyle.Render(summary(it.typ.Doc)), mutedStyle.Render("type"))
	case itemError:
		line = columns(width, "  "+errStyle.Render("!")+" "+textStyle.Render(it.err.Name), mutedStyle.Render(summary(it.err.Doc)), mutedStyle.Render("error"))
	}
	if !selected {
		return "  " + line
	}
	st := selectedStyle
	if focused {
		st = selectedFocusedStyle
	}
	return brandStyle.Render("▌ ") + st.Width(width).Render(ansi.Strip(line))
}

var typeGlyph = render.TypeStyle.Render("τ")

// counts summarizes an interface as "3 methods · 2 types · 1 error", leaving out zeros.
func counts(p *idl.Interface) string {
	parts := []string{fmt.Sprintf("%d %s", len(p.Methods), plural(len(p.Methods), "method", "methods"))}
	if n := len(p.Types); n > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", n, plural(n, "type", "types")))
	}
	if n := len(p.Errors); n > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", n, plural(n, "error", "errors")))
	}
	return strings.Join(parts, " · ")
}

// summary is the first sentence of a doc comment, on one line.
func summary(doc string) string {
	doc = strings.Join(strings.Fields(doc), " ")
	if i := strings.Index(doc, ". "); i >= 0 {
		doc = doc[:i+1]
	}
	return doc
}

// columns lays out a left part, a middle that takes the slack and a right
// part, in width cells.
func columns(width int, left, middle, right string) string {
	lw, rw := ansi.StringWidth(left), ansi.StringWidth(right)
	nameCol := max(lw, min(28, width/3))
	if nameCol+rw+4 > width {
		left = ansi.Truncate(left, max(width-rw-2, 4), "…")
		lw = ansi.StringWidth(left)
		nameCol = lw
		middle = ""
	}
	mid := max(width-nameCol-rw-4, 0)
	middle = ansi.Truncate(middle, mid, "…")
	gap := width - nameCol - 2 - ansi.StringWidth(middle) - rw
	return left + strings.Repeat(" ", nameCol-lw+2) + middle + strings.Repeat(" ", max(gap, 1)) + right
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
