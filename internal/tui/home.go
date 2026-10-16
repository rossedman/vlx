package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/sanitize"
)

func (m *Model) viewHome(p *page, width, height int) string {
	if !m.discovered || len(m.sockets) == 0 {
		return m.welcome(width)
	}
	where := ""
	if m.host != "" {
		where = " on " + sanitize.Line(m.host)
	}
	intro := mutedStyle.Render(fmt.Sprintf("%d Varlink %s%s. Pick one, or type below to jump straight to anything.",
		len(m.sockets), plural(len(m.sockets), "service", "services"), where))
	lines := []string{intro, ""}
	lines = append(lines, m.renderList(p, m.items(p), width-2, height-2, !m.cmd.focused)...)
	return strings.Join(lines, "\n")
}

func (m *Model) socketLine(s discovery.Socket, width int) string {
	_, desc := describe(s)
	dot := faintStyle.Render("○")
	meta := ""
	st := m.state[s.Path]
	switch {
	case !s.Accessible:
		meta = warnStyle.Render("⊘ needs root")
		if m.root {
			meta = warnStyle.Render("⊘ no access")
		}
	case st != nil && st.loading:
		dot = m.spinner.View()
		meta = mutedStyle.Render("reading…")
	case st != nil && st.err != nil:
		dot = errStyle.Render("✗")
		meta = errStyle.Render("✗ could not connect")
	case m.cache[s.Path] != nil:
		dot = okStyle.Render("●")
		n := 0
		for _, i := range m.cache[s.Path].Interfaces {
			if i.Parsed != nil && !boilerplate(i.Name) {
				n += len(i.Parsed.Methods)
			}
		}
		meta = mutedStyle.Render(fmt.Sprintf("%d %s", n, plural(n, "method", "methods")))
	}
	if desc == "" {
		desc = s.Path
	}
	return columns(width, dot+" "+boldStyle.Render(sanitize.Line(label(s))), textStyle.Render(sanitize.Line(desc)), meta)
}

func (m *Model) historyLine(h historyEntry, width int) string {
	mark := m.spinner.View()
	switch {
	case h.done && h.ok:
		mark = okStyle.Render("✓")
	case h.done:
		mark = errStyle.Render("✗")
	}
	s, _ := socketFor(h.path, m.sockets)
	name := mark + " " + boldStyle.Render(sanitize.Line(discovery.ShortName(s.Name)+" › "+h.method))
	mid := textStyle.Render(sanitize.Line(h.args)) + "  " + mutedStyle.Render(sanitize.Line(h.status))
	return columns(width, name, mid, mutedStyle.Render(h.at.Format("15:04")))
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

// welcome is the home page while discovery runs or when it finds nothing.
func (m *Model) welcome(width int) string {
	var b strings.Builder
	b.WriteString("\n")
	for i, l := range logo {
		b.WriteString(lipgloss.NewStyle().Foreground(logoColors[i]).Render(l) + "\n")
	}
	b.WriteString(mutedStyle.Render("Varlink explorer") + "\n\n")
	if !m.discovered {
		b.WriteString(m.spinner.View() + mutedStyle.Render(" Looking for Varlink sockets…"))
		return b.String()
	}
	b.WriteString(wrap(textStyle.Render("No Varlink sockets were found in the usual places."), width) + "\n\n")
	b.WriteString(wrap(mutedStyle.Render("systemd v257 or newer serves its APIs under /run/systemd. Try:"), width) + "\n\n")
	b.WriteString("  " + brandStyle.Render("vlx demo") + mutedStyle.Render("          a sample service") + "\n")
	b.WriteString("  " + brandStyle.Render("vlx PATH") + mutedStyle.Render("          a socket anywhere") + "\n")
	b.WriteString("  " + brandStyle.Render("--search DIR") + mutedStyle.Render("      look somewhere else") + "\n\n")
	b.WriteString(wrap(mutedStyle.Render("You can also type a socket path below and press ⏎."), width))
	return b.String()
}

func modeExplanation(writes bool) string {
	if writes {
		return "Writes are enabled: vlx asks before every WRITE call and makes you type the name of DANGER methods."
	}
	return "vlx is read-only: you can prepare any call, but only READ methods are sent. Restart with --allow-writes to change that."
}

func wrap(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

func heading(s string) string {
	return sectionStyle.Render(s)
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func lower(s string) string { return strings.ToLower(s) }
