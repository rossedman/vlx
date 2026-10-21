package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
)

var (
	logoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(render.BrandDeep).
			Bold(true).
			Padding(0, 1)
	mutedStyle = lipgloss.NewStyle().Foreground(render.Muted)
	faintStyle = lipgloss.NewStyle().Foreground(render.Faint)
	textStyle  = lipgloss.NewStyle().Foreground(render.Text)
	boldStyle  = lipgloss.NewStyle().Foreground(render.Text).Bold(true)
	brandStyle = lipgloss.NewStyle().Foreground(render.Brand).Bold(true)
	okStyle    = lipgloss.NewStyle().Foreground(render.Green)
	warnStyle  = lipgloss.NewStyle().Foreground(render.Amber)
	errStyle   = lipgloss.NewStyle().Foreground(render.Red)

	selectedStyle = lipgloss.NewStyle().
			Foreground(render.Text).
			Background(render.Surface).
			Bold(true)
	selectedFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(render.BrandDeep).
				Bold(true)

	sectionStyle = lipgloss.NewStyle().Foreground(render.Brand).Bold(true).MarginTop(1)
	docStyle     = lipgloss.NewStyle().Foreground(render.Text)

	dialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			Padding(1, 3)

	flashStyle = lipgloss.NewStyle().Foreground(render.Green)
)

// badge renders label as a solid block of colour fg, as in the header.
func badge(label string, fg lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0F1117"}).
		Background(fg).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

// pill renders label as bold text in fg.
func pill(label string, fg lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Foreground(fg).Bold(true).Render(label)
}

// classColor is the colour of a safety class: green READ, amber WRITE,
// red for anything more.
func classColor(c safety.Class) lipgloss.TerminalColor {
	switch c {
	case safety.Read:
		return render.Green
	case safety.Write:
		return render.Amber
	default:
		return render.Red
	}
}

// classTag renders a safety class's name in its colour.
func classTag(c safety.Class) string {
	return pill(c.String(), classColor(c))
}
