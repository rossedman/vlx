package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
)

func (m *Model) updateHelp(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "q":
		if m.helpReturn == screenBrowse {
			return tea.Quit
		}
		m.screen = m.helpReturn
	case "esc", "?", "f1", "enter":
		m.screen = m.helpReturn
	}
	return nil
}

func helpSection(title string, rows [][2]string) string {
	var b strings.Builder
	b.WriteString(sectionStyle.MarginTop(0).Render(title) + "\n")
	for _, r := range rows {
		b.WriteString("  " + lipgloss.NewStyle().Foreground(render.Brand).Bold(true).Width(12).Render(r[0]) + mutedStyle.Render(r[1]) + "\n")
	}
	return b.String()
}

func bindingRows(bs ...key.Binding) [][2]string {
	rows := make([][2]string, len(bs))
	for i, b := range bs {
		rows[i] = [2]string{b.Help().Key, b.Help().Desc}
	}
	return rows
}

func (m *Model) viewHelp() string {
	browse := helpSection("Browse", bindingRows(
		bk.Palette, bk.Up, bk.Down, bk.Next, bk.Prev, bk.Left, bk.Right,
		key.NewBinding(key.WithHelp("⏎", "open service · expand · call")),
		bk.Toggle, bk.Call, bk.IDL, bk.Filter, bk.Copy, bk.Refresh, bk.Top, bk.Bottom, bk.Back, bk.Quit,
	))
	call := helpSection("Call", bindingRows(
		ck.Send, ck.Switch, ck.Mode, ck.More, ck.Oneway, ck.Reset, ck.CopyCmd, ck.CopyResp, ck.Reveal,
		key.NewBinding(key.WithHelp("esc", "stop stream · back")),
		ck.Quit,
	))
	glyphs := helpSection("Symbols", [][2]string{
		{"ƒ", "method, single reply"},
		{"⌁", "method that streams"},
		{"τ", "type"},
		{"!", "error"},
		{"● ○ ✗", "connected · not opened · failed"},
		{"⊘", "no permission to connect"},
	})
	sf := helpSection("Safety", [][2]string{
		{safety.Read.String(), "no side effects; always allowed"},
		{safety.Write.String(), "changes state; needs --allow-writes + y"},
		{safety.Danger.String(), "disruptive; needs --allow-writes + typed name"},
	})
	mode := sectionStyle.MarginTop(0).Render("Mode") + "\n" + wrap(mutedStyle.Render(modeExplanation(m.policy.AllowWrites)), 44)

	colW := 48
	left := lipgloss.NewStyle().Width(colW).Render(browse + "\n" + glyphs)
	right := lipgloss.NewStyle().Width(colW).Render(call + "\n" + sf + "\n" + mode)
	var body string
	if m.width >= colW*2+4 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	} else {
		body = lipgloss.JoinVertical(lipgloss.Left, left, right)
	}
	footer := mutedStyle.Render("esc returns · docs: https://rossedman.github.io/vlx")
	content := lipgloss.JoinVertical(lipgloss.Left, body, "", footer)
	return lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Top, content)
}
