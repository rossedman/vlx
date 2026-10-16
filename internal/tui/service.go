package tui

import (
	"strings"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/sanitize"
)

func (m *Model) viewService(p *page, width, height int) string {
	s, _ := socketFor(p.path, m.sockets)
	_, desc := describe(s)
	title := boldStyle.Render(sanitize.Line(label(s)))
	if desc != "" {
		title += mutedStyle.Render("  " + sanitize.Line(desc))
	}
	sub := sanitize.Line(p.path)
	svc := m.cache[p.path]
	if svc != nil {
		var who []string
		for _, v := range []string{svc.Info.Vendor, svc.Info.Product, svc.Info.Version} {
			if v != "" {
				who = append(who, sanitize.Line(v))
			}
		}
		if len(who) > 0 {
			sub += " · " + strings.Join(who, " ")
		}
	}
	lines := []string{title, faintStyle.Render(sub), ""}

	st := m.state[p.path]
	switch {
	case svc != nil:
		lines = append(lines, m.renderList(p, m.items(p), width-2, height-len(lines), !m.cmd.focused)...)
	case st != nil && st.err != nil:
		lines = append(lines, wrap(errStyle.Render("✗ "+sanitize.Line(st.err.Error())), width))
		if hint := client.Hint(st.err); hint != "" {
			lines = append(lines, "", wrap(mutedStyle.Render(hint), width))
		}
		lines = append(lines, "", mutedStyle.Render("ctrl+r tries again · esc goes back"))
	case !s.Accessible:
		lines = append(lines, wrap(warnStyle.Render("⊘ You do not have permission to connect to this socket. Try sudo."), width))
	default:
		lines = append(lines, m.spinner.View()+mutedStyle.Render(" Reading the service's interfaces (read-only introspection)…"))
	}
	return strings.Join(lines, "\n")
}
