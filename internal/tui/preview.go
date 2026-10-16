package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rossedman/vlx/internal/client"
)

// previewDelay is how long the cursor must rest on a service before vlx
// introspects it, so scrolling through the list sends nothing.
var previewDelay = 250 * time.Millisecond

// previewMsg asks to introspect path if the cursor is still on it.
type previewMsg struct {
	seq  int
	path string
}

// showService makes the socket at path the current service: from the
// cache if it was introspected before, otherwise (with preview enabled)
// by scheduling a read-only introspection once the cursor rests on it.
func (m *Model) showService(path string) tea.Cmd {
	if path == "" || (path == m.svcPath && m.svc != nil) {
		return nil
	}
	m.svcPath = path
	m.showIDL = false
	if svc := m.cache[path]; svc != nil {
		m.setService(svc)
		return nil
	}
	m.svc = nil
	m.rows = nil
	m.refreshDetail()
	if !m.cfg.Preview {
		return nil
	}
	if st := m.state[path]; st != nil && (st.loading || st.err != nil) {
		return nil
	}
	if s, _ := socketFor(path, m.sockets); !s.Accessible {
		return nil
	}
	m.previewSeq++
	seq := m.previewSeq
	return tea.Tick(previewDelay, func(time.Time) tea.Msg { return previewMsg{seq: seq, path: path} })
}

func (m *Model) handlePreview(msg previewMsg) tea.Cmd {
	if msg.seq != m.previewSeq || msg.path != m.svcPath || m.cache[msg.path] != nil {
		return nil
	}
	if st := m.state[msg.path]; st != nil && st.loading {
		return nil
	}
	return m.introspect(msg.path)
}

// introspect starts a read-only introspection of path.
func (m *Model) introspect(path string) tea.Cmd {
	m.state[path] = &sockState{loading: true}
	return tea.Batch(introspectCmd(path, m.copts), m.spinner.Tick)
}

// setService shows an introspected service in the tree.
func (m *Model) setService(svc *client.Service) {
	m.svc = svc
	m.expanded = map[string]bool{}
	for _, i := range svc.Interfaces {
		m.expanded[i.Name] = !boilerplate(i.Name)
	}
	m.memFilter.SetValue("")
	m.rebuildRows()
	m.memCursor, m.memOffset = 0, 0
	for i, r := range m.rows {
		if r.kind == rowMethod {
			m.memCursor = i
			break
		}
	}
	m.refreshDetail()
}

// boilerplate interfaces are implemented by every (systemd) service and
// start collapsed.
func boilerplate(name string) bool {
	return name == "org.varlink.service" || name == "io.systemd.service" || name == "io.systemd"
}

// openSelected moves into the tree of the selected service, introspecting
// it first if needed.
func (m *Model) openSelected() tea.Cmd {
	s, ok := m.selectedSocket()
	if !ok {
		return nil
	}
	cmd := m.showService(s.Path)
	if m.svc != nil {
		m.focus = paneMembers
		m.layout()
		return cmd
	}
	m.openOnLoad = s.Path
	if st := m.state[s.Path]; st != nil && st.loading {
		return nil
	}
	return m.introspect(s.Path)
}
