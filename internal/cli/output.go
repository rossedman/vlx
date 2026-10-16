package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/charmbracelet/lipgloss"

	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/sanitize"
)

var (
	headStyle  = lipgloss.NewStyle().Foreground(render.Muted).Bold(true)
	nameStyle  = lipgloss.NewStyle().Foreground(render.Text).Bold(true)
	mutedStyle = lipgloss.NewStyle().Foreground(render.Muted)
	warnStyle  = lipgloss.NewStyle().Foreground(render.Amber)
	errStyle   = lipgloss.NewStyle().Foreground(render.Red).Bold(true)
)

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// table writes aligned columns. Styles are applied after alignment so that
// escape codes do not upset tabwriter.
type table struct {
	w    io.Writer
	rows [][]string
}

func newTable(w io.Writer, header ...string) *table {
	t := &table{w: w}
	t.rows = append(t.rows, header)
	return t
}

func (t *table) add(cells ...string) {
	for i := range cells {
		cells[i] = sanitize.Line(cells[i])
	}
	t.rows = append(t.rows, cells)
}

func (t *table) flush(color bool, colStyles ...lipgloss.Style) error {
	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 0, 3, ' ', 0)
	for _, r := range t.rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(b.String(), "\n"), "\n")
	for i, line := range lines {
		if color {
			if i == 0 {
				line = headStyle.Render(line)
			} else if len(colStyles) > 0 {
				line = styleFirstColumn(line, len(t.rows[i][0]), colStyles[0])
			}
		}
		if _, err := fmt.Fprintln(t.w, strings.TrimRight(line, " ")); err != nil {
			return err
		}
	}
	return nil
}

func styleFirstColumn(line string, n int, st lipgloss.Style) string {
	if n > len(line) {
		return line
	}
	return st.Render(line[:n]) + line[n:]
}
