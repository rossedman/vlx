package tui

import (
	"sort"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/sanitize"
)

// entryKind is what a search result is.
type entryKind int

const (
	entryService entryKind = iota
	entryIface
	entryMethod
	entryType
	entryError
)

// entry is one thing the command line can jump to.
type entry struct {
	kind    entryKind
	path    string // socket
	service string // socket name, e.g. io.systemd.Resolve
	iface   string
	name    string // member name, or the service's short name
	more    bool
	verdict safety.Verdict
	score   int
}

// qualified is the entry's full name: the service name, the interface name,
// or interface.member.
func (e entry) qualified() string {
	switch e.kind {
	case entryService:
		return e.service
	case entryIface:
		return e.iface
	}
	return e.iface + "." + e.name
}

// entries lists everything searchable: every discovered service, and the
// interfaces and members of every introspected one.
func (m *Model) entries() []entry {
	var out []entry
	for _, s := range m.sockets {
		out = append(out, entry{kind: entryService, path: s.Path, service: s.Name, name: discovery.ShortName(s.Name)})
		svc := m.cache[s.Path]
		if svc == nil {
			continue
		}
		for _, iface := range svc.Interfaces {
			base := entry{path: s.Path, service: s.Name, iface: iface.Name}
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
func matchScore(query string, e entry) (int, bool) {
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
			case ti == 0 && subsequence(lower, tok):
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

// isBoundary reports whether s[i] starts a word: it follows a separator or
// is the upper-case start of a CamelCase hump. i must be at least 1.
func isBoundary(s string, i int) bool {
	prev := rune(s[i-1])
	cur := rune(s[i])
	return prev == '.' || prev == '_' || prev == '-' || prev == ' ' ||
		(unicode.IsUpper(cur) && unicode.IsLower(prev))
}

// subsequence reports whether the bytes of sub appear in s in order, not
// necessarily together.
func subsequence(s, sub string) bool {
	j := 0
	for i := 0; i < len(s) && j < len(sub); i++ {
		if s[i] == sub[j] {
			j++
		}
	}
	return j == len(sub)
}

// entryRow renders a suggestion: a glyph for its kind, its name, what it is
// and, for methods, the safety class; highlighted when selected.
func (m *Model) entryRow(e entry, width int, selected bool) string {
	s, _ := socketFor(e.path, m.sockets)
	svc := sanitize.Line(label(s))
	var glyph, name, what, tag string
	switch e.kind {
	case entryService:
		glyph, name = okStyle.Render("●"), svc
		_, desc := describe(s)
		what = "service · " + desc
	case entryIface:
		glyph, name, what = brandStyle.Render("▸"), svc+" › "+e.iface, "interface"
	case entryMethod:
		glyph = brandStyle.Render("ƒ")
		if e.more {
			glyph = brandStyle.Render("⌁")
		}
		name, what, tag = svc+" › "+e.name, "method · "+m.signature(e), classTag(e.verdict.Class)
	case entryType:
		glyph, name, what = lipgloss.NewStyle().Foreground(render.Cyan).Render("τ"), svc+" › "+e.name, "type in "+e.iface
	case entryError:
		glyph, name, what = errStyle.Render("!"), svc+" › "+e.name, "error in "+e.iface
	}
	line := columns(width, glyph+" "+boldStyle.Render(sanitize.Line(name)), mutedStyle.Render(sanitize.Line(what)), tag)
	if selected {
		return selectedFocusedStyle.Width(width).Render(ansi.Strip(line))
	}
	return line
}

// signature summarises a method's parameters, like "name string, family ?int".
func (m *Model) signature(e entry) string {
	svc := m.cache[e.path]
	if svc == nil {
		return ""
	}
	iface := svc.Interface(e.iface)
	if iface == nil || iface.Parsed == nil {
		return ""
	}
	mt := iface.Parsed.Method(e.name)
	if mt == nil || mt.In == nil || len(mt.In.Fields) == 0 {
		return "no parameters"
	}
	parts := make([]string, len(mt.In.Fields))
	for i, f := range mt.In.Fields {
		parts[i] = f.Name + " " + f.Type.String()
	}
	return strings.Join(parts, ", ")
}

// search returns the entries matching query, best first.
func (m *Model) search(query string, limit int) []entry {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil
	}
	var res []entry
	for _, e := range m.entries() {
		if s, ok := matchScore(q, e); ok {
			e.score = s
			res = append(res, e)
		}
	}
	sort.SliceStable(res, func(i, j int) bool {
		if res[i].score != res[j].score {
			return res[i].score > res[j].score
		}
		return res[i].qualified() < res[j].qualified()
	})
	if len(res) > limit {
		res = res[:limit]
	}
	return res
}
