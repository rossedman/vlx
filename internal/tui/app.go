// Package tui is vlx's interactive explorer, built with Bubble Tea.
//
// The browse screen has three panes — services, interfaces and detail —
// and the call screen pairs a schema-aware parameter editor with a live
// response view. All socket I/O runs in tea.Cmds so the UI never blocks.
package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
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

type pane int

const (
	paneServices pane = iota
	paneMembers
	paneDetail
)

type screen int

const (
	screenBrowse screen = iota
	screenCall
	screenHelp
)

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

	width, height int
	screen        screen
	helpReturn    screen
	focus         pane

	sockets    []discovery.Socket
	discovered bool
	state      map[string]*sockState
	svcCursor  int
	svcOffset  int
	svcFilter  textinput.Model

	svc        *client.Service
	svcPath    string
	cache      map[string]*client.Service
	previewSeq int
	openOnLoad string // move into the tree when this path finishes loading
	expanded   map[string]bool
	rows       []row
	memCursor  int
	memOffset  int
	memFilter  textinput.Model

	filtering bool // the focused pane's filter input has the keyboard

	detail  viewport.Model
	showIDL bool

	palette *palette

	call   *callState
	drafts map[string]string
	callID int

	spinner spinner.Model
	help    help.Model
	flash   string
	flashOK bool
	flashID int
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
		expanded:   map[string]bool{},
		drafts:     map[string]string{},
		detail:     viewport.New(40, 10),
		help:       help.New(),
		width:      100,
		height:     30,
	}
	m.svcFilter = newFilterInput()
	m.memFilter = newFilterInput()
	m.spinner = spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(lipgloss.NewStyle().Foreground(render.Brand)))
	m.help.Styles.ShortKey = lipgloss.NewStyle().Foreground(render.Brand).Bold(true)
	m.help.Styles.ShortDesc = mutedStyle
	m.help.Styles.ShortSeparator = faintStyle
	m.help.Styles.FullKey = m.help.Styles.ShortKey
	m.help.Styles.FullDesc = mutedStyle
	if opts.Target != "" {
		m.svcPath = opts.Target
		m.openOnLoad = opts.Target
	}
	return m
}

func newFilterInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.PromptStyle = lipgloss.NewStyle().Foreground(render.Brand)
	ti.Placeholder = "filter"
	ti.CharLimit = 64
	staticCursorInTests(&ti.Cursor)
	return ti
}

// Init starts discovery, and introspection of the target if there is one.
func (m *Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.SetWindowTitle("vlx — Varlink explorer"),
		discoverCmd(m.opts.Discovery),
		m.spinner.Tick,
	}
	if m.opts.Target != "" {
		m.state[m.opts.Target] = &sockState{loading: true}
		cmds = append(cmds, introspectCmd(m.opts.Target, m.copts))
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
		if m.busy() && m.call != nil && m.screen == screenCall {
			m.refreshResponse(false)
		}
		return m, cmd

	case discoveredMsg:
		return m, m.setSockets(msg.sockets)

	case previewMsg:
		return m, m.handlePreview(msg)

	case introspectedMsg:
		cmd := m.handleIntrospected(msg)
		m.refreshPalette()
		return m, cmd

	case replyMsg:
		if m.call != nil && msg.id == m.call.id {
			m.call.addReply(msg.reply, m.cfg.MaxReplies, m.renderReply)
			m.refreshResponse(true)
		}
		return m, waitFor(msg.ch)

	case callDoneMsg:
		if m.call != nil && msg.id == m.call.id {
			m.call.finish(msg.err)
			m.refreshResponse(true)
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
			m.stopCall()
			return m, tea.Quit
		}
		switch m.screen {
		case screenHelp:
			return m, m.updateHelp(msg)
		case screenCall:
			return m, m.updateCall(msg)
		default:
			return m, m.updateBrowse(msg)
		}
	}
	if m.palette != nil {
		var cmd tea.Cmd
		m.palette.input, cmd = m.palette.input.Update(msg)
		return m, cmd
	}
	if m.screen == screenCall && m.call != nil {
		return m, m.call.forward(msg)
	}
	return m, nil
}

func (m *Model) busy() bool {
	if m.call != nil && m.call.running {
		return true
	}
	for _, st := range m.state {
		if st.loading {
			return true
		}
	}
	return false
}

func (m *Model) setFlash(text string, ok bool) tea.Cmd {
	m.flashID++
	m.flash = sanitize.Line(text)
	m.flashOK = ok
	return clearFlashAfter(m.flashID, 4*time.Second)
}

func (m *Model) stopCall() {
	if m.call != nil && m.call.cancel != nil {
		m.call.cancel()
	}
}

func (m *Model) setSockets(sockets []discovery.Socket) tea.Cmd {
	var selected string
	if s, ok := m.selectedSocket(); ok {
		selected = s.Path
	}
	if m.svcPath != "" {
		if _, found := socketFor(m.svcPath, sockets); !found {
			s, _ := socketFor(m.svcPath, nil)
			sockets = append([]discovery.Socket{s}, sockets...)
		}
		if selected == "" {
			selected = m.svcPath
		}
	}
	m.sockets = sockets
	m.discovered = true
	m.svcCursor = 0
	for i, idx := range m.visibleSockets() {
		if m.sockets[idx].Path == selected {
			m.svcCursor = i
		}
	}
	m.refreshDetail()
	if m.svcPath == "" {
		if s, ok := m.selectedSocket(); ok {
			return m.showService(s.Path)
		}
	}
	return nil
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
	opening := m.openOnLoad == msg.path
	if opening {
		m.openOnLoad = ""
	}
	if msg.path != m.svcPath {
		return nil
	}
	if msg.err != nil {
		m.svc = nil
		m.rows = nil
		m.focus = paneServices
		m.layout()
		if opening {
			return m.setFlash("Could not open "+msg.path+": "+msg.err.Error(), false)
		}
		return nil
	}
	m.setService(msg.svc)
	if opening {
		m.focus = paneMembers
		m.layout()
	}
	return nil
}

// layout sizes the sub-models after a resize.
func (m *Model) layout() {
	_, _, dw := m.paneWidths()
	m.detail.Width = max(dw-4, 10)
	m.detail.Height = max(m.bodyHeight()-3, 3)
	m.help.Width = m.width - 1
	if m.call != nil {
		m.layoutCall()
	}
	m.refreshDetail()
}

func (m *Model) bodyHeight() int {
	return max(m.height-3, 5) // header + status + help
}

// paneWidths returns the outer widths of the three panes; 0 hides a pane.
func (m *Model) paneWidths() (services, members, detail int) {
	w := m.width
	if w < 100 {
		if m.focus == paneServices {
			services = w * 2 / 5
			return services, w - services, 0
		}
		members = w * 2 / 5
		return 0, members, w - members
	}
	services = clamp(w/5, 24, 34)
	members = clamp(w*3/10, 32, 48)
	return services, members, w - services - members
}

func clamp(v, lo, hi int) int {
	return max(lo, min(v, hi))
}

// View renders the current screen.
func (m *Model) View() string {
	if m.width < 60 || m.height < 16 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			warnStyle.Render(fmt.Sprintf("vlx needs at least 60×16 (now %d×%d)", m.width, m.height)))
	}
	var body string
	switch m.screen {
	case screenHelp:
		body = m.viewHelp()
	case screenCall:
		body = m.viewCall()
	default:
		body = m.viewBrowse()
	}
	if m.palette != nil && m.screen == screenBrowse {
		body = lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, m.viewPalette())
	}
	if m.call != nil && m.call.confirm != confirmNone && m.screen == screenCall {
		body = lipgloss.Place(m.width, m.bodyHeight(), lipgloss.Center, lipgloss.Center, m.viewConfirm())
	}
	return lipgloss.JoinVertical(lipgloss.Left, m.viewHeader(), body, m.viewStatus(), m.viewFooter())
}

func (m *Model) viewHeader() string {
	left := logoStyle.Render("vlx")
	title := "Varlink explorer"
	if m.svcPath != "" {
		title = m.svcPath
	}
	if m.opts.Note != "" {
		title += mutedStyle.Render("  " + m.opts.Note)
	}
	left += " " + headerPathStyle.Render(title)

	var badges []string
	if m.policy.AllowWrites {
		badges = append(badges, badge("WRITES ENABLED", render.Amber))
	} else {
		badges = append(badges, badge("READ-ONLY", render.Green))
	}
	if m.root {
		badges = append(badges, badge("ROOT", render.Red))
	}
	right := strings.Join(badges, " ")
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
	case m.flash != "":
		st := flashStyle
		if !m.flashOK {
			st = errStyle
		}
		s = st.Render(m.flash)
	default:
		for path, st := range m.state {
			if st.loading {
				s = m.spinner.View() + mutedStyle.Render(" Connecting to "+path+" …")
				break
			}
		}
	}
	return ansi.Truncate(" "+s, m.width, "…")
}

func (m *Model) viewFooter() string {
	var keys []key.Binding
	switch m.screen {
	case screenHelp:
		keys = []key.Binding{bk.Back, bk.Quit}
	case screenCall:
		keys = m.callHelpKeys()
	default:
		keys = m.browseHelpKeys()
	}
	// help truncates with an ellipsis, but not in every case; clamp anyway.
	return ansi.Truncate(" "+m.help.ShortHelpView(keys), m.width, "…")
}
