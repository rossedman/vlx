// Package tui is vlx's interactive explorer, built with Bubble Tea.
//
// It shows one page at a time — the services, one service, one method —
// with a command line along the bottom that is always there: type a few
// letters to jump anywhere, or a whole call such as
// `greeter Hello name=Ada` to run it. On a method page the parameter form
// and the command line show the same call, and editing either updates the
// other. All socket I/O runs in tea.Cmds so the UI never blocks.
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/sanitize"
)

// Options configures the explorer.
type Options struct {
	Config *config.Config
	// Target is a socket path to open on start, or "".
	Target string
	// Discovery configures socket discovery.
	Discovery discovery.Options
	// Note is shown in the header, e.g. "demo".
	Note string
}

type sockState struct {
	loading bool
	ok      bool
	err     error
}

// Model is the Bubble Tea model of the explorer.
type Model struct {
	opts       Options
	cfg        *config.Config
	classifier *safety.Classifier
	policy     safety.Policy
	redactor   *render.Redactor
	copts      client.Options
	root       bool
	host       string

	width, height int

	sockets    []discovery.Socket
	discovered bool
	state      map[string]*sockState
	cache      map[string]*client.Service
	previewSeq int

	stack []*page

	cmd command

	calls   map[string]*callState // by socket path and qualified method
	history []historyEntry
	callID  int

	spinner spinner.Model
	flash   string
	flashOK bool
	flashID int
}

// historyEntry is one call sent in this session.
type historyEntry struct {
	id        int
	path      string
	qualified string
	method    string
	line      string // the command line that repeats it
	args      string // its arguments, for display
	status    string
	ok        bool
	done      bool
	at        time.Time
}

// New returns the explorer model.
func New(opts Options) *Model {
	cfg := opts.Config
	m := &Model{
		opts:       opts,
		cfg:        cfg,
		classifier: safety.NewClassifier(cfg.Safety),
		policy:     safety.Policy{AllowWrites: cfg.AllowWrites},
		redactor:   render.NewRedactor(cfg.Redact),
		copts:      client.Options{Timeout: cfg.Timeout, MaxFrameSize: cfg.MaxFrameSize},
		root:       os.Geteuid() == 0,
		state:      map[string]*sockState{},
		cache:      map[string]*client.Service{},
		calls:      map[string]*callState{},
		width:      100,
		height:     30,
	}
	m.host, _ = os.Hostname()
	m.spinner = spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(render.Brand)))
	m.cmd = newCommand()
	m.stack = []*page{newPage(pageHome, "")}
	if opts.Target != "" {
		m.push(newPage(pageService, opts.Target))
	}
	return m
}

// Init starts discovery, and introspection of the target if there is one.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.SetWindowTitle("vlx — Varlink explorer"),
		discoverCmd(m.opts.Discovery),
		m.spinner.Tick,
		textinput.Blink,
	}
	if m.opts.Target != "" {
		cmds = append(cmds, m.introspect(m.opts.Target))
	}
	return tea.Batch(cmds...)
}

// Update handles a message.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if c := m.currentCall(); c != nil && c.running {
			m.refreshResponse(c, false)
		}
		return m, cmd

	case discoveredMsg:
		return m, m.setSockets(msg.sockets)

	case previewMsg:
		return m, m.handlePreview(msg)

	case introspectedMsg:
		return m, m.handleIntrospected(msg)

	case replyMsg:
		if c := m.callByID(msg.id); c != nil {
			c.addReply(msg.reply, m.cfg.MaxReplies, m.renderReply)
			m.refreshResponse(c, true)
		}
		return m, waitFor(msg.ch)

	case callDoneMsg:
		if c := m.callByID(msg.id); c != nil {
			c.finish(msg.err)
			m.recordDone(c)
			m.refreshResponse(c, true)
		}
		return m, nil

	case flashMsg:
		return m, m.setFlash(msg.text, !msg.err)

	case flashClearMsg:
		if msg.id == m.flashID {
			m.flash = ""
		}
		return m, nil

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.stopAll()
			return m, tea.Quit
		}
		return m, m.handleKey(msg)
	}
	// Cursor blinks and the like go to whatever has the keyboard.
	if m.cmd.focused {
		var cmd tea.Cmd
		m.cmd.input, cmd = m.cmd.input.Update(msg)
		return m, cmd
	}
	if c := m.currentCall(); c != nil {
		return m, c.forward(msg)
	}
	return m, nil
}

// handleKey routes a key: a confirmation dialog first, then the command
// line if it has the keyboard, then the page.
func (m *Model) handleKey(msg tea.KeyMsg) tea.Cmd {
	if c := m.currentCall(); c != nil && c.confirm != confirmNone {
		return m.updateConfirm(c, msg)
	}
	if msg.String() == "f1" {
		return m.openHelp()
	}
	if m.cmd.focused {
		return m.updateCommand(msg)
	}
	p := m.page()
	switch p.kind {
	case pageMethod:
		return m.updateMethod(p, msg)
	case pageDetail, pageHelp:
		return m.updateReader(p, msg)
	default:
		return m.updateList(p, msg)
	}
}

func (m *Model) setFlash(text string, ok bool) tea.Cmd {
	m.flashID++
	m.flash = sanitize.Line(text)
	m.flashOK = ok
	return clearFlashAfter(m.flashID, flashFor)
}

// flashFor is how long a status message stays.
var flashFor = 4 * time.Second

func (m *Model) stopAll() {
	for _, c := range m.calls {
		if c.cancel != nil {
			c.cancel()
		}
	}
}

func (m *Model) setSockets(sockets []discovery.Socket) tea.Cmd {
	if t := m.opts.Target; t != "" {
		if _, found := socketFor(t, sockets); !found {
			s, _ := socketFor(t, nil)
			sockets = append([]discovery.Socket{s}, sockets...)
		}
	}
	m.sockets = sockets
	m.discovered = true
	if m.page().kind == pageHome {
		return m.previewSelected()
	}
	return nil
}

// introspect starts a read-only introspection of path.
func (m *Model) introspect(path string) tea.Cmd {
	if st := m.state[path]; st != nil && st.loading {
		return nil
	}
	m.state[path] = &sockState{loading: true}
	return tea.Batch(introspectCmd(path, m.copts), m.spinner.Tick)
}

// ensure introspects path unless it is cached or being read.
func (m *Model) ensure(path string) tea.Cmd {
	if m.cache[path] != nil {
		return nil
	}
	return m.introspect(path)
}

func (m *Model) handleIntrospected(msg introspectedMsg) tea.Cmd {
	st := m.state[msg.path]
	if st == nil {
		st = &sockState{}
		m.state[msg.path] = st
	}
	st.loading = false
	st.err = msg.err
	st.ok = msg.err == nil
	if msg.err == nil {
		m.cache[msg.path] = msg.svc
	}
	var cmds []tea.Cmd
	if m.cmd.pendingPath == msg.path {
		line := m.cmd.pendingLine
		m.cmd.pendingPath, m.cmd.pendingLine = "", ""
		if msg.err != nil {
			cmds = append(cmds, m.setFlash("Could not open "+msg.path+": "+msg.err.Error(), false))
		} else {
			cmds = append(cmds, m.runLine(line, false))
		}
	}
	m.refreshSuggestions()
	return tea.Batch(cmds...)
}

// layout sizes the sub-models after a resize.
func (m *Model) layout() {
	m.cmd.input.Width = max(m.width-8, 10)
	for _, p := range m.stack {
		m.layoutPage(p)
	}
}

const (
	gutter      = 2 // columns of space left and right of the body
	chromeLines = 6 // header, a blank line, status, rule, command line, key hints
)

func (m *Model) bodyHeight() int { return max(m.height-chromeLines, 5) }
func (m *Model) bodyWidth() int  { return max(m.width-2*gutter, 20) }

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

// View renders the current page with the command line beneath it.
func (m *Model) View() string {
	if m.width < 60 || m.height < 16 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			warnStyle.Render(fmt.Sprintf("vlx needs at least 60×16 (now %d×%d)", m.width, m.height)))
	}
	p := m.page()
	bh, bw := m.bodyHeight(), m.bodyWidth()
	var body string
	switch p.kind {
	case pageHome:
		body = m.viewHome(p, bw, bh)
	case pageService:
		body = m.viewService(p, bw, bh)
	case pageMethod:
		body = m.viewMethod(p, bw, bh)
	default:
		body = m.viewReader(p, bw, bh)
	}
	lines := fitLines(strings.Split(body, "\n"), bh, bw)
	if c := m.currentCall(); c != nil && c.confirm != confirmNone {
		lines = strings.Split(lipgloss.Place(bw, bh, lipgloss.Center, lipgloss.Center, m.viewConfirm(c)), "\n")
	} else if pop := m.viewSuggestions(bw); pop != "" {
		lines = overlayBottom(lines, strings.Split(pop, "\n"))
	}
	pad := strings.Repeat(" ", gutter)
	for i, l := range lines {
		lines[i] = pad + l
	}
	return strings.Join([]string{
		m.viewHeader(),
		"",
		strings.Join(lines, "\n"),
		m.viewStatus(),
		faintStyle.Render(strings.Repeat("─", m.width)),
		m.viewCommand(),
		m.viewHints(),
	}, "\n")
}

// fitLines pads or cuts lines to exactly h lines no wider than w.
func fitLines(lines []string, h, w int) []string {
	out := make([]string, 0, h)
	for _, l := range lines {
		if len(out) == h {
			break
		}
		out = append(out, ansi.Truncate(l, w, "…"))
	}
	for len(out) < h {
		out = append(out, "")
	}
	return out
}

// overlayBottom replaces the last lines of body with pop.
func overlayBottom(body, pop []string) []string {
	if len(pop) > len(body) {
		pop = pop[len(pop)-len(body):]
	}
	copy(body[len(body)-len(pop):], pop)
	return body
}

func (m *Model) viewHeader() string {
	parts := []string{logoStyle.Render("vlx")}
	crumbs := m.breadcrumb()
	for i, c := range crumbs {
		st := mutedStyle
		if i == len(crumbs)-1 {
			st = boldStyle
		}
		parts = append(parts, faintStyle.Render("›"), st.Render(sanitize.Line(c)))
	}
	left := " " + strings.Join(parts, " ")
	if m.opts.Note != "" {
		left += mutedStyle.Render("  " + m.opts.Note)
	}

	var badges []string
	if m.policy.AllowWrites {
		badges = append(badges, badge("WRITES ENABLED", render.Amber))
	} else {
		badges = append(badges, badge("READ-ONLY", render.Green))
	}
	if m.root {
		badges = append(badges, badge("ROOT", render.Red))
	}
	right := strings.Join(badges, " ") + " "
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		left = ansi.Truncate(left, max(m.width-lipgloss.Width(right)-2, 0), "…")
		gap = max(m.width-lipgloss.Width(left)-lipgloss.Width(right), 1)
	}
	return left + strings.Repeat(" ", gap) + right
}

func (m *Model) viewStatus() string {
	var s string
	switch {
	case m.cmd.err != "" && m.cmd.focused:
		s = errStyle.Render("✗ " + m.cmd.err)
	case m.flash != "":
		st := flashStyle
		if !m.flashOK {
			st = errStyle
		}
		s = st.Render(m.flash)
	case m.cmd.pendingPath != "":
		s = m.spinner.View() + mutedStyle.Render(" Connecting to "+m.cmd.pendingPath+" …")
	default:
		for path, st := range m.state {
			if st.loading && m.page().path == path {
				s = m.spinner.View() + mutedStyle.Render(" Connecting to "+path+" …")
				break
			}
		}
	}
	return ansi.Truncate(strings.Repeat(" ", gutter)+s, m.width, "…")
}
