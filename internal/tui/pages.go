package tui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
)

type pageKind int

const (
	pageHome pageKind = iota
	pageService
	pageMethod
	pageDetail
	pageHelp
)

type methodFocus int

const (
	focusForm methodFocus = iota
	focusResult
)

// page is one screen in the drill-down stack.
type page struct {
	kind pageKind
	path string // the service's socket, for every page but home and help

	// List pages: the selected item by key, and the scroll offset.
	sel      string
	offset   int
	expanded map[string]bool

	// Method page.
	call  *callState
	focus methodFocus

	// Detail and help pages.
	title  string
	item   item
	idl    bool
	reader viewport.Model
}

// newPage makes a page of the given kind for the service at path.
func newPage(kind pageKind, path string) *page {
	return &page{kind: kind, path: path, expanded: map[string]bool{}, reader: viewport.New(40, 10)}
}

// page is the page on top of the stack, the one being shown.
func (m *Model) page() *page { return m.stack[len(m.stack)-1] }

// push puts p on top of the stack, sized for the screen.
func (m *Model) push(p *page) {
	m.stack = append(m.stack, p)
	m.layoutPage(p)
}

// pop goes up a level; home has no parent.
func (m *Model) pop() tea.Cmd {
	if len(m.stack) == 1 {
		return nil
	}
	m.page().blur()
	m.stack = m.stack[:len(m.stack)-1]
	m.cmd.clear()
	p := m.page()
	if p.kind == pageMethod {
		m.syncCommand()
		p.focus = focusForm
		return p.call.focusEditor()
	}
	if p.kind == pageHome {
		return m.previewSelected()
	}
	return nil
}

// goTo replaces everything above home with the given pages.
func (m *Model) goTo(pages ...*page) {
	m.page().blur()
	m.stack = m.stack[:1]
	for _, p := range pages {
		m.push(p)
	}
}

// blur takes the keyboard from a method page's editor as the page is left.
func (p *page) blur() {
	if p.kind == pageMethod {
		p.call.blurEditor()
	}
}

// layoutPage sizes a page for the screen; only method, detail and help
// pages have anything to size.
func (m *Model) layoutPage(p *page) {
	switch p.kind {
	case pageMethod:
		m.layoutMethod(p)
	case pageDetail, pageHelp:
		p.reader.Width = m.bodyWidth()
		p.reader.Height = max(m.bodyHeight()-2, 3)
		m.refreshReader(p)
	}
}

// breadcrumb names the pages on the stack.
func (m *Model) breadcrumb() []string {
	out := []string{"Services"}
	for _, p := range m.stack[1:] {
		switch p.kind {
		case pageService:
			s, _ := socketFor(p.path, m.sockets)
			out = append(out, label(s))
		case pageMethod:
			out = append(out, p.call.method.Name)
		default:
			out = append(out, p.title)
		}
	}
	return out
}

// currentCall is the call on the current page, if it is a method page.
func (m *Model) currentCall() *callState {
	if p := m.page(); p.kind == pageMethod {
		return p.call
	}
	return nil
}

// callByID finds a call by the id its replies carry, or nil if it is gone.
func (m *Model) callByID(id int) *callState {
	for _, c := range m.calls {
		if c.id == id {
			return c
		}
	}
	return nil
}

// openService shows the service page for path, introspecting it if
// needed.
func (m *Model) openService(path, iface string) tea.Cmd {
	p := newPage(pageService, path)
	if iface != "" {
		p.expanded[iface] = true
		p.sel = "i:" + iface
	}
	m.goTo(p)
	m.cmd.clear()
	return m.ensure(path)
}

// openMethod shows the method page for a method of an introspected
// service, keeping what was typed and sent there before.
func (m *Model) openMethod(path string, iface *client.Interface, mt *idl.Method) tea.Cmd {
	key := path + "\x00" + iface.Name + "." + mt.Name
	c := m.calls[key]
	if c == nil {
		c = newCallState(path, iface, mt)
		m.calls[key] = c
	}
	sp := newPage(pageService, path)
	sp.expanded[iface.Name] = true
	sp.sel = "m:" + iface.Name + "." + mt.Name
	mp := newPage(pageMethod, path)
	mp.call = c
	m.goTo(sp, mp)
	m.refreshResponse(c, false)
	m.syncCommand()
	return c.focusEditor()
}

// openDetail shows a page about a type, an error, a method's
// documentation or an interface's IDL.
func (m *Model) openDetail(path string, it item, idlSource bool) {
	p := newPage(pageDetail, path)
	p.item = it
	p.idl = idlSource
	p.title = it.name()
	m.page().blur()
	m.push(p)
	m.cmd.clear()
}

// openHelp shows the help page, or closes it if it is already showing.
func (m *Model) openHelp() tea.Cmd {
	if m.page().kind == pageHelp {
		return m.pop()
	}
	p := newPage(pageHelp, "")
	p.title = "Help"
	m.page().blur()
	m.cmd.blur()
	m.push(p)
	return nil
}

// previewDelay is how long the cursor must rest on a service before vlx
// introspects it, so scrolling through the list sends nothing.
var previewDelay = 250 * time.Millisecond

// previewMsg asks to introspect path if the cursor is still on it.
type previewMsg struct {
	seq  int
	path string
}

// previewSelected schedules a read-only introspection of the service
// under the cursor on the home page, when preview is on.
func (m *Model) previewSelected() tea.Cmd {
	p := m.page()
	if p.kind != pageHome || !m.cfg.Preview {
		return nil
	}
	it, ok := m.selected(p)
	if !ok || it.kind != itemSocket || !it.sock.Accessible || m.cache[it.sock.Path] != nil {
		return nil
	}
	if st := m.state[it.sock.Path]; st != nil {
		return nil // being read, or failed before
	}
	m.previewSeq++
	seq, path := m.previewSeq, it.sock.Path
	return tea.Tick(previewDelay, func(time.Time) tea.Msg { return previewMsg{seq: seq, path: path} })
}

// handlePreview introspects the previewed service, unless the cursor has
// moved on since the preview was scheduled.
func (m *Model) handlePreview(msg previewMsg) tea.Cmd {
	if msg.seq != m.previewSeq {
		return nil
	}
	return m.ensure(msg.path)
}

// indexAll introspects every reachable service so that the command line
// can search all of them. It runs once, when preview is on, on the first
// keystroke in the command line.
func (m *Model) indexAll() tea.Cmd {
	if !m.cfg.Preview {
		return nil
	}
	var cmds []tea.Cmd
	for _, s := range m.sockets {
		if !s.Accessible || m.cache[s.Path] != nil || m.state[s.Path] != nil {
			continue
		}
		cmds = append(cmds, m.introspect(s.Path))
	}
	return tea.Batch(cmds...)
}

// indexing counts the services still being introspected.
func (m *Model) indexing() int {
	n := 0
	for _, st := range m.state {
		if st.loading {
			n++
		}
	}
	return n
}

// boilerplate interfaces are implemented by every (systemd) service and
// start collapsed.
func boilerplate(name string) bool {
	return name == "org.varlink.service" || name == "io.systemd.service" || name == "io.systemd"
}

// isExpanded reports whether an interface is open on a service page;
// boilerplate interfaces start closed, the rest open.
func (p *page) isExpanded(iface string) bool {
	if v, ok := p.expanded[iface]; ok {
		return v
	}
	return !boilerplate(iface)
}

// svcToken is the shortest word that names path on the command line.
func (m *Model) svcToken(path string) string {
	s, _ := socketFor(path, m.sockets)
	last := s.Name[strings.LastIndexByte(s.Name, '.')+1:]
	for _, cand := range []string{lower(last), lower(discovery.ShortName(s.Name)), s.Name} {
		if cand == "" {
			continue
		}
		if got, err := discovery.Resolve(cand, m.sockets); err == nil && got == path {
			return cand
		}
	}
	return path
}
