// Package render draws Varlink data for people: colourised JSON with
// redaction, highlighted IDL and the shared colour palette.
package render

import "github.com/charmbracelet/lipgloss"

// Palette. Every colour is an adaptive light/dark pair.
var (
	Brand     = lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#A78BFA"}
	BrandDeep = lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#7C3AED"}
	Green     = lipgloss.AdaptiveColor{Light: "#0E8A47", Dark: "#30D475"}
	Amber     = lipgloss.AdaptiveColor{Light: "#A15C00", Dark: "#F5B83D"}
	Red       = lipgloss.AdaptiveColor{Light: "#C62A31", Dark: "#F2545B"}
	Cyan      = lipgloss.AdaptiveColor{Light: "#0E7490", Dark: "#22D3EE"}
	Pink      = lipgloss.AdaptiveColor{Light: "#BE185D", Dark: "#F472B6"}
	Text      = lipgloss.AdaptiveColor{Light: "#1F1B24", Dark: "#E9E6EE"}
	Muted     = lipgloss.AdaptiveColor{Light: "#6B6576", Dark: "#8A8496"}
	Faint     = lipgloss.AdaptiveColor{Light: "#C9C3D3", Dark: "#3A3546"}
	Surface   = lipgloss.AdaptiveColor{Light: "#F4F1F7", Dark: "#1A1622"}
)

// Styles for syntax highlighting, shared by the TUI and the CLI.
var (
	KeyStyle     = lipgloss.NewStyle().Foreground(Cyan)
	StringStyle  = lipgloss.NewStyle().Foreground(Green)
	NumberStyle  = lipgloss.NewStyle().Foreground(Amber)
	BoolStyle    = lipgloss.NewStyle().Foreground(Pink)
	NullStyle    = lipgloss.NewStyle().Foreground(Muted).Italic(true)
	PunctStyle   = lipgloss.NewStyle().Foreground(Muted)
	RedactStyle  = lipgloss.NewStyle().Foreground(Red).Italic(true)
	CommentStyle = lipgloss.NewStyle().Foreground(Muted).Italic(true)
	KeywordStyle = lipgloss.NewStyle().Foreground(Brand).Bold(true)
	NameStyle    = lipgloss.NewStyle().Foreground(Text).Bold(true)
	TypeStyle    = lipgloss.NewStyle().Foreground(Cyan)
	ErrorStyle   = lipgloss.NewStyle().Foreground(Red).Bold(true)
)
