package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/sanitize"
)

// refreshReader regenerates a detail or help page's text.
func (m *Model) refreshReader(p *page) {
	width := max(p.reader.Width, 20)
	var content string
	switch {
	case p.kind == pageHelp:
		content = m.helpContent(width)
	case p.idl:
		it := p.item
		content = render.IDL(it.iface.Description, true)
		if it.iface.Err != nil && it.iface.Description == "" {
			content = errStyle.Render(sanitize.Text(it.iface.Err.Error()))
		}
	case p.item.kind == itemMethod:
		var b strings.Builder
		m.detailMethod(&b, p.item, width)
		content = b.String()
	default:
		content = m.detailRow(p.item, width)
	}
	p.reader.SetContent(strings.TrimRight(content, "\n"))
}

// updateReader handles a key on a detail or help page: going back, toggling
// docs and IDL, jumping to the command line, or scrolling.
func (m *Model) updateReader(p *page, msg tea.KeyMsg) tea.Cmd {
	switch {
	case msg.String() == "q" && p.kind == pageHelp:
		m.stopAll()
		return tea.Quit
	case msg.String() == "?" || key.Matches(msg, lk.Back):
		return m.pop()
	case key.Matches(msg, lk.Docs) && p.kind == pageDetail:
		p.idl = !p.idl
		m.refreshReader(p)
		p.reader.GotoTop()
		return nil
	case key.Matches(msg, gk.Command), p.kind == pageDetail && isTyping(msg):
		cmd := m.cmd.focus()
		if isTyping(msg) {
			return tea.Batch(cmd, m.updateCommand(msg))
		}
		return cmd
	case key.Matches(msg, lk.Top):
		p.reader.GotoTop()
		return nil
	case key.Matches(msg, lk.Bottom):
		p.reader.GotoBottom()
		return nil
	}
	var cmd tea.Cmd
	p.reader, cmd = p.reader.Update(msg)
	return cmd
}

// viewReader renders a detail or help page: a title line, with the docs and
// IDL switch on detail pages, over the scrolling text.
func (m *Model) viewReader(p *page, width, height int) string {
	var title string
	if p.kind == pageHelp {
		title = boldStyle.Render("Help") + mutedStyle.Render("  everything vlx's keys do")
	} else {
		kind := map[itemKind]string{itemIface: "interface", itemMethod: "method", itemType: "type", itemError: "error"}[p.item.kind]
		title = mutedStyle.Render(kind+" ") + boldStyle.Render(sanitize.Line(p.item.name()))
		docs, src := brandStyle.Render("docs"), faintStyle.Render("IDL")
		if p.idl {
			docs, src = faintStyle.Render("docs"), brandStyle.Render("IDL")
		}
		if p.item.kind != itemIface {
			title += mutedStyle.Render("  in " + sanitize.Line(p.item.iface.Name))
		}
		title += "   " + docs + faintStyle.Render(" · ") + src + faintStyle.Render("  ctrl+d switches")
	}
	return title + "\n\n" + p.reader.View()
}
