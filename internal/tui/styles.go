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
	headerPathStyle = lipgloss.NewStyle().Foreground(render.Text).Bold(true)
	mutedStyle      = lipgloss.NewStyle().Foreground(render.Muted)
	faintStyle      = lipgloss.NewStyle().Foreground(render.Faint)
	textStyle       = lipgloss.NewStyle().Foreground(render.Text)
	boldStyle       = lipgloss.NewStyle().Foreground(render.Text).Bold(true)
	brandStyle      = lipgloss.NewStyle().Foreground(render.Brand).Bold(true)
	okStyle         = lipgloss.NewStyle().Foreground(render.Green)
	warnStyle       = lipgloss.NewStyle().Foreground(render.Amber)
	errStyle        = lipgloss.NewStyle().Foreground(render.Red)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(render.Faint).
			Padding(0, 1)
	paneFocusedStyle = paneStyle.BorderForeground(render.Brand)

	paneTitleStyle        = lipgloss.NewStyle().Foreground(render.Muted).Bold(true)
	paneTitleFocusedStyle = lipgloss.NewStyle().Foreground(render.Brand).Bold(true)

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

func badge(label string, fg lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.AdaptiveColor{Light: "#FFFFFF", Dark: "#0F1117"}).
		Background(fg).
		Bold(true).
		Padding(0, 1).
		Render(label)
}

func pill(label string, fg lipgloss.TerminalColor) string {
	return lipgloss.NewStyle().Foreground(fg).Bold(true).Render(label)
}

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

func classTag(c safety.Class) string {
	return pill(c.String(), classColor(c))
}
