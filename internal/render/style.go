package render

// lipglossStyle is the subset of lipgloss.Style the writers use.
type lipglossStyle interface {
	Render(...string) string
}
