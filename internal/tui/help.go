package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/lipgloss"

	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
)

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

// helpContent is the text of the help page.
func (m *Model) helpContent(width int) string {
	cmd := helpSection("Command line", [][2]string{
		{"type", "start typing anywhere to search"},
		{"/ : ctrl+k", "go to the command line"},
		{"⏎", "open the match, or run the call"},
		{"⇥", "complete the selected match"},
		{"↑ ↓", "pick a match · earlier calls"},
		{"esc", "leave the command line"},
	})
	grammar := sectionStyle.MarginTop(0).Render("Calls") + "\n" +
		"  " + textStyle.Render("SERVICE [METHOD] [NAME=VALUE …]") + "\n" +
		"  " + brandStyle.Render("demo GetGreeting name=Ada") + "\n" +
		"  " + brandStyle.Render("resolve ResolveHostname name=example.com") + "\n" +
		lipgloss.NewStyle().PaddingLeft(2).Width(48).Render(mutedStyle.Render("Quote values with spaces: name='Ada Lovelace'. Strings and enums are taken as typed; numbers, booleans and structs are JSON. On a method page the form and the line stay in sync.")) + "\n"
	lists := helpSection("Pages", bindingRows(lk.Up, lk.Down, lk.Open, lk.Toggle, lk.Back, lk.Docs, lk.Copy, lk.Refresh))
	lists += helpSection("", [][2]string{{"?  f1", "this help"}, {"q", "quit (ctrl+c anywhere)"}})
	method := helpSection("Method page", bindingRows(mk.Send, mk.Next, mk.Mode, mk.More, mk.Oneway, mk.Reset, mk.Copy, mk.Docs, mk.PageUp, mk.CopyReply, mk.Reveal, mk.Back))
	glyphs := helpSection("Symbols", [][2]string{
		{"ƒ", "method, single reply"},
		{"⌁", "method that streams"},
		{"τ", "type"},
		{"!", "error"},
		{"● ○ ✗", "connected · not read yet · failed"},
		{"⊘", "no permission to connect"},
	})
	sf := helpSection("Safety", [][2]string{
		{safety.Read.String(), "no side effects; always allowed"},
		{safety.Write.String(), "changes state; needs --allow-writes + y"},
		{safety.Danger.String(), "disruptive; needs --allow-writes + typed name"},
	})
	mode := sectionStyle.MarginTop(0).Render("Mode") + "\n" + wrap(mutedStyle.Render(modeExplanation(m.policy.AllowWrites)), 46)

	colW := 50
	left := lipgloss.NewStyle().Width(colW).Render(cmd + "\n" + grammar + "\n" + lists)
	right := lipgloss.NewStyle().Width(colW).Render(method + "\n" + glyphs + "\n" + sf + "\n" + mode)
	body := lipgloss.JoinVertical(lipgloss.Left, left, right)
	if width >= colW*2+2 {
		body = lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", right)
	}
	return body + "\n\n" + mutedStyle.Render("Docs: https://rossedman.github.io/vlx")
}
