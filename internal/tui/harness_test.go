package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/spf13/viper"

	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/demo"
	"github.com/rossedman/vlx/internal/discovery"
)

// harness drives a Model synchronously: it runs commands, feeds their
// messages back, and drops timer ticks so tests do not loop forever.
type harness struct {
	t *testing.T
	m *Model
}

func init() {
	noBlink = true
	previewDelay = time.Millisecond
	flashFor = time.Millisecond
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	v := viper.New()
	config.SetDefaults(v)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(v, "")
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// newHarness opens the explorer on a demo service, as `vlx demo` does.
func newHarness(t *testing.T, allowWrites bool) (*harness, string) {
	t.Helper()
	srv, err := demo.ListenTemp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	cfg := testConfig(t)
	cfg.AllowWrites = allowWrites
	h := start(t, Options{
		Config:    cfg,
		Target:    srv.Path(),
		Discovery: discovery.Options{Dirs: []string{}, Extra: []string{filepath.Dir(srv.Path())}},
		Note:      "demo service",
	}, 140, 40)
	return h, srv.Path()
}

// newBrowseHarness starts demo services in one directory and opens the
// explorer without a target, as plain `vlx` would.
func newBrowseHarness(t *testing.T, preview bool, names ...string) (*harness, string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "vlx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	if len(names) == 0 {
		names = []string{"io.vlx.demo", "com.example.Beta"}
	}
	for _, name := range names {
		srv, err := demo.Listen(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { srv.Close() })
	}
	cfg := testConfig(t)
	cfg.Preview = preview
	return start(t, Options{Config: cfg, Discovery: discovery.Options{Dirs: []string{dir}}}, 140, 40), dir
}

func start(t *testing.T, opts Options, w, h int) *harness {
	t.Helper()
	m := New(opts)
	screenshotMode(m)
	m.host = "atlas"
	hs := &harness{t: t, m: m}
	hs.send(tea.WindowSizeMsg{Width: w, Height: h})
	hs.run(m.Init())
	return hs
}

// screenshotMode renders in true colour on a dark background, as a
// terminal would, when screenshots are being captured.
func screenshotMode(m *Model) {
	if os.Getenv("VLX_SCREENSHOT_DIR") != "" {
		lipgloss.SetColorProfile(termenv.TrueColor)
		lipgloss.SetHasDarkBackground(true)
		m.root = false
	}
}

func (h *harness) run(cmd tea.Cmd) {
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		done := make(chan tea.Msg, 1)
		go func() { done <- c() }()
		var msg tea.Msg
		select {
		case msg = <-done:
		case <-time.After(800 * time.Millisecond):
			continue // a timer (flash expiry, cursor blink): abandon it
		}
		switch msg := msg.(type) {
		case nil, spinner.TickMsg, flashClearMsg:
			continue
		case tea.BatchMsg:
			queue = append(queue, msg...)
			continue
		}
		if name := fmtType(msg); strings.HasPrefix(name, "tea.") || strings.HasPrefix(name, "cursor.") {
			continue // window title, cursor blinks and other timers
		}
		_, next := h.m.Update(msg)
		queue = append(queue, next)
	}
}

func (h *harness) send(msg tea.Msg) {
	_, cmd := h.m.Update(msg)
	h.run(cmd)
}

var namedKeys = map[string]tea.KeyType{
	"enter": tea.KeyEnter, "esc": tea.KeyEsc, "tab": tea.KeyTab, "shift+tab": tea.KeyShiftTab,
	"up": tea.KeyUp, "down": tea.KeyDown, "left": tea.KeyLeft, "right": tea.KeyRight,
	"pgup": tea.KeyPgUp, "pgdown": tea.KeyPgDown, "home": tea.KeyHome, "end": tea.KeyEnd,
	"backspace": tea.KeyBackspace, "f1": tea.KeyF1, "ctrl+c": tea.KeyCtrlC,
	"ctrl+s": tea.KeyCtrlS, "ctrl+t": tea.KeyCtrlT, "ctrl+o": tea.KeyCtrlO, "ctrl+r": tea.KeyCtrlR,
	"ctrl+j": tea.KeyCtrlJ, "ctrl+y": tea.KeyCtrlY, "ctrl+d": tea.KeyCtrlD, "ctrl+k": tea.KeyCtrlK,
	"ctrl+p": tea.KeyCtrlP, "ctrl+n": tea.KeyCtrlN,
}

func keyFor(k string) tea.KeyMsg {
	if t, ok := namedKeys[k]; ok {
		return tea.KeyMsg{Type: t}
	}
	if k == " " {
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}

// keys sends named keys, or runes for anything else.
func (h *harness) keys(keys ...string) {
	for _, k := range keys {
		h.send(keyFor(k))
	}
}

// typ types text one character at a time.
func (h *harness) typ(s string) {
	for _, r := range s {
		h.keys(string(r))
	}
}

func (h *harness) view() string {
	return ansi.Strip(h.m.View())
}

func (h *harness) contains(want ...string) {
	h.t.Helper()
	v := h.view()
	for _, w := range want {
		if !strings.Contains(v, w) {
			h.t.Fatalf("view lacks %q:\n%s", w, v)
		}
	}
}

func (h *harness) lacks(bad ...string) {
	h.t.Helper()
	v := h.view()
	for _, w := range bad {
		if strings.Contains(v, w) {
			h.t.Fatalf("view has %q:\n%s", w, v)
		}
	}
}

func (h *harness) screenshot(name string) {
	dir := os.Getenv("VLX_SCREENSHOT_DIR")
	if dir == "" {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, name+".ans"), []byte(h.m.View()), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

// checkFits fails if the view is not exactly width×height.
func checkFits(t *testing.T, v string, width, height int) {
	t.Helper()
	lines := strings.Split(v, "\n")
	if len(lines) != height {
		t.Fatalf("view has %d lines, want %d:\n%s", len(lines), height, ansi.Strip(v))
	}
	for i, l := range lines {
		if w := ansi.StringWidth(l); w > width {
			t.Fatalf("line %d is %d wide, max %d: %q", i, w, width, ansi.Strip(l))
		}
	}
}

// discovery0 finds nothing.
func discovery0() discovery.Options { return discovery.Options{Dirs: []string{}} }
