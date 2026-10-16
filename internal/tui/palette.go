package tui

import (
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/sanitize"
)

// entryKind extends rowKind with whole services.
type entryKind int

const (
	entryService entryKind = iota
	entryIface
	entryMethod
	entryType
	entryError
)

// paletteEntry is one thing the palette can jump to.
type paletteEntry struct {
	kind    entryKind
	path    string // socket
	service string // socket name, e.g. io.systemd.Resolve
	iface   string
	name    string // member name, or the service's short name
	more    bool
	verdict safety.Verdict
	score   int
}

func (e paletteEntry) qualified() string {
	switch e.kind {
	case entryService:
		return e.service
	case entryIface:
		return e.iface
	}
	return e.iface + "." + e.name
}

// palette is the search-anything overlay.
type palette struct {
	input   textinput.Model
	results []paletteEntry
	cursor  int
	offset  int
}

func newPalette() *palette {
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = brandStyle
	ti.Placeholder = "search services, methods, types, errors… e.g. resolve host"
	ti.CharLimit = 128
	staticCursorInTests(&ti.Cursor)
	ti.Focus()
	return &palette{input: ti}
}

// openPalette shows the palette and, with preview enabled, introspects
// every reachable service in the background so that all of them can be
// searched.
func (m *Model) openPalette() tea.Cmd {
	m.palette = newPalette()
	m.refreshPalette()
	cmds := []tea.Cmd{textinput.Blink}
	if m.cfg.Preview {
		for _, s := range m.sockets {
			if !s.Accessible || m.cache[s.Path] != nil {
				continue
			}
			if st := m.state[s.Path]; st != nil && (st.loading || st.err != nil) {
				continue
			}
			cmds = append(cmds, m.introspect(s.Path))
		}
	}
	return tea.Batch(cmds...)
}

// indexing counts the services still being introspected.
func (m *Model) indexing() int {
	n := 0
	for _, st := range m.state {
		if st.loading {
			n++
		}
	}
	return n
}

// entries lists everything searchable: every discovered service, and the
// interfaces and members of every introspected one.
func (m *Model) entries() []paletteEntry {
	var out []paletteEntry
	for _, s := range m.sockets {
		out = append(out, paletteEntry{kind: entryService, path: s.Path, service: s.Name, name: discovery.ShortName(s.Name)})
		svc := m.cache[s.Path]
		if svc == nil {
			continue
		}
		for _, iface := range svc.Interfaces {
			base := paletteEntry{path: s.Path, service: s.Name, iface: iface.Name}
			e := base
			e.kind, e.name = entryIface, iface.Name
			out = append(out, e)
			p := iface.Parsed
			if p == nil {
				continue
			}
			for _, mt := range p.Methods {
				e := base
				e.kind, e.name, e.more = entryMethod, mt.Name, mt.More != idl.MoreNone
				e.verdict = m.classifier.Classify(iface.Name + "." + mt.Name)
				out = append(out, e)
			}
			for _, t := range p.Types {
				e := base
				e.kind, e.name = entryType, t.Name
				out = append(out, e)
			}
			for _, er := range p.Errors {
				e := base
				e.kind, e.name = entryError, er.Name
				out = append(out, e)
			}
		}
	}
	return out
}

// matchScore scores how well query matches an entry; ok is false when it
// does not match. Every whitespace-separated token must match, as a
// substring (best at the start of a word or a CamelCase hump) or, failing
// that, as a subsequence.
func matchScore(query string, e paletteEntry) (int, bool) {
	tokens := strings.Fields(strings.ToLower(query))
	targets := []string{e.name, e.qualified(), discovery.ShortName(e.service)}
	score := 0
	for _, tok := range tokens {
		best, found := 0, false
		for ti, target := range targets {
			lower := strings.ToLower(target)
			idx := strings.Index(lower, tok)
			s := 0
			switch {
			case idx >= 0:
				s = 10
				if idx == 0 || isBoundary(target, idx) {
					s += 8
				}
				if ti == 0 {
					s += 4 // matches the name itself
				}
			case subsequence(lower, tok):
				s = 2
			default:
				continue
			}
			if !found || s > best {
				best, found = s, true
			}
		}
		if !found {
			return 0, false
		}
		score += best
	}
	if strings.EqualFold(strings.Join(tokens, " "), e.name) {
		score += 12 // the whole query is exactly this name
	}
	switch e.kind {
	case entryMethod:
		score += 6
	case entryService:
		score += 4
	case entryIface:
		score += 3
	case entryType:
		score++
	}
	score -= len(e.name) / 8
	return score, true
}

func isBoundary(s string, i int) bool {
	prev := rune(s[i-1])
	cur := rune(s[i])
	return prev == '.' || prev == '_' || prev == '-' || prev == ' ' ||
		(unicode.IsUpper(cur) && unicode.IsLower(prev))
}

func subsequence(s, sub string) bool {
	j := 0
	for i := 0; i < len(s) && j < len(sub); i++ {
		if s[i] == sub[j] {
			j++
		}
	}
	return j == len(sub)
}

const paletteMax = 200

// refreshPalette recomputes the results for the current query.
func (m *Model) refreshPalette() {
	p := m.palette
	if p == nil {
		return
	}
	q := strings.TrimSpace(p.input.Value())
	var res []paletteEntry
	for _, e := range m.entries() {
		if q == "" {
			if e.kind == entryService {
				res = append(res, e)
			}
			continue
		}
		if s, ok := matchScore(q, e); ok {
			e.score = s
			res = append(res, e)
		}
	}
	if q != "" {
		sort.SliceStable(res, func(i, j int) bool {
			if res[i].score != res[j].score {
				return res[i].score > res[j].score
			}
			return res[i].qualified() < res[j].qualified()
		})
	}
	if len(res) > paletteMax {
		res = res[:paletteMax]
	}
	p.results = res
	p.cursor = clamp(p.cursor, 0, max(len(res)-1, 0))
}

func (m *Model) updatePalette(msg tea.KeyMsg) tea.Cmd {
	p := m.palette
	switch msg.String() {
	case "esc", "ctrl+k":
		m.palette = nil
		return nil
	case "up", "ctrl+p", "shift+tab":
		p.cursor = max(p.cursor-1, 0)
		return nil
	case "down", "ctrl+n", "tab":
		p.cursor = min(p.cursor+1, max(len(p.results)-1, 0))
		return nil
	case "enter":
		if p.cursor < len(p.results) {
			e := p.results[p.cursor]
			m.palette = nil
			return m.jumpTo(e)
		}
		return nil
	}
	before := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if p.input.Value() != before {
		p.cursor, p.offset = 0, 0
		m.refreshPalette()
	}
	return cmd
}

// jumpTo shows an entry: its service, and the entry selected in the tree.
func (m *Model) jumpTo(e paletteEntry) tea.Cmd {
	m.svcFilter.SetValue("")
	for i, idx := range m.visibleSockets() {
		if m.sockets[idx].Path == e.path {
			m.svcCursor = i
		}
	}
	if e.kind == entryService {
		return m.openSelected()
	}
	cmd := m.showService(e.path)
	if m.svc == nil {
		return tea.Batch(cmd, m.openSelected())
	}
	m.expanded[e.iface] = true
	m.memFilter.SetValue("")
	m.rebuildRows()
	for i, r := range m.rows {
		if r.iface.Name != e.iface {
			continue
		}
		match := false
		switch e.kind {
		case entryIface:
			match = r.kind == rowIface
		case entryMethod:
			match = r.kind == rowMethod && r.method.Name == e.name
		case entryType:
			match = r.kind == rowType && r.typ.Name == e.name
		case entryError:
			match = r.kind == rowError && r.err.Name == e.name
		}
		if match {
			m.memCursor = i
			break
		}
	}
	m.focus = paneMembers
	m.showIDL = false
	m.layout()
	return cmd
}

func (m *Model) viewPalette() string {
	p := m.palette
	w := min(m.width-6, 96)
	h := min(m.bodyHeight()-2, 22)
	inner := w - 6
	p.input.Width = inner - 4

	var lines []string
	lines = append(lines, brandStyle.Render("Search everything"), "", p.input.View(), faintStyle.Render(strings.Repeat("─", inner)))
	listH := h - 4 - 4 - 2
	if len(p.results) == 0 {
		lines = append(lines, mutedStyle.Render("No matches."))
	}
	start, end := listWindow(p.cursor, len(p.results), listH, &p.offset)
	for i := start; i < end; i++ {
		lines = append(lines, m.paletteRow(p.results[i], inner, i == p.cursor))
	}
	for len(lines) < h-4-2 {
		lines = append(lines, "")
	}
	status := faintStyle.Render(fmt.Sprintf("%d %s", len(p.results), plural(len(p.results), "result", "results")))
	if n := m.indexing(); n > 0 {
		status = m.spinner.View() + mutedStyle.Render(fmt.Sprintf(" reading %d more %s…", n, plural(n, "service", "services")))
	} else if !m.cfg.Preview {
		status = faintStyle.Render("preview is off: only opened services are searched")
	}
	lines = append(lines, "", status)
	return dialogStyle.BorderForeground(render.Brand).Padding(1, 2).Width(w).Render(strings.Join(lines, "\n"))
}

func (m *Model) paletteRow(e paletteEntry, width int, selected bool) string {
	var glyph, tag string
	switch e.kind {
	case entryService:
		glyph = okStyle.Render("●")
		tag = mutedStyle.Render("service")
	case entryIface:
		glyph = brandStyle.Render("▸")
		tag = mutedStyle.Render("interface")
	case entryMethod:
		glyph = brandStyle.Render("ƒ")
		if e.more {
			glyph = brandStyle.Render("⌁")
		}
		tag = classTag(e.verdict.Class)
	case entryType:
		glyph = lipgloss.NewStyle().Foreground(render.Cyan).Render("τ")
		tag = mutedStyle.Render("type")
	case entryError:
		glyph = errStyle.Render("!")
		tag = mutedStyle.Render("error")
	}
	where := discovery.ShortName(e.service)
	if e.kind != entryService && e.kind != entryIface {
		where = e.iface + " · " + where
	} else if e.kind == entryService {
		where = e.service
	}
	left := glyph + " " + boldStyle.Render(sanitize.Line(e.name)) + "  " + mutedStyle.Render(sanitize.Line(where))
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(tag)
	if gap < 2 {
		left = ansi.Truncate(left, max(width-ansi.StringWidth(tag)-3, 4), "…")
		gap = max(width-ansi.StringWidth(left)-ansi.StringWidth(tag), 1)
	}
	line := left + strings.Repeat(" ", gap) + tag
	if selected {
		return selectedFocusedStyle.Width(width).Render(ansi.Strip(line))
	}
	return line
}
