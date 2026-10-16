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

func newHarness(t *testing.T, allowWrites bool) (*harness, string) {
	t.Helper()
	srv, err := demo.ListenTemp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	v := viper.New()
	config.SetDefaults(v)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(v, "")
	if err != nil {
		t.Fatal(err)
	}
	cfg.AllowWrites = allowWrites
	m := New(Options{
		Config:    cfg,
		Target:    srv.Path(),
		Discovery: discovery.Options{Dirs: []string{}, Extra: []string{filepath.Dir(srv.Path())}},
		Note:      "demo service",
	})
	if os.Getenv("VLX_SCREENSHOT_DIR") != "" {
		lipgloss.SetColorProfile(termenv.TrueColor)
		lipgloss.SetHasDarkBackground(true)
		m.root = false
	}
	h := &harness{t: t, m: m}
	h.send(tea.WindowSizeMsg{Width: 140, Height: 40})
	h.run(m.Init())
	return h, srv.Path()
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
		if name := typeName(msg); strings.HasPrefix(name, "tea.") || strings.HasPrefix(name, "cursor.") {
			continue // window title, cursor blinks and other timers
		}
		_, next := h.m.Update(msg)
		queue = append(queue, next)
	}
}

func typeName(v any) string {
	return strings.TrimPrefix(strings.TrimPrefix(fmtType(v), "*"), "")
}

func (h *harness) send(msg tea.Msg) {
	_, cmd := h.m.Update(msg)
	h.run(cmd)
}

func (h *harness) keys(keys ...string) {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "ctrl+s":
			msg = tea.KeyMsg{Type: tea.KeyCtrlS}
		case "ctrl+t":
			msg = tea.KeyMsg{Type: tea.KeyCtrlT}
		case "ctrl+o":
			msg = tea.KeyMsg{Type: tea.KeyCtrlO}
		case "ctrl+r":
			msg = tea.KeyMsg{Type: tea.KeyCtrlR}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		h.send(msg)
	}
}

func (h *harness) view() string {
	return ansi.Strip(h.m.View())
}

// selectMethod moves the interfaces cursor to the named method.
func (h *harness) selectMethod(name string) {
	h.t.Helper()
	for i, r := range h.m.rows {
		if r.kind == rowMethod && r.method.Name == name {
			h.m.memCursor = i
			h.m.focus = paneMembers
			h.m.refreshDetail()
			return
		}
	}
	h.t.Fatalf("method %s not in tree", name)
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

func TestBrowseDemo(t *testing.T) {
	h, path := newHarness(t, false)
	if h.m.svc == nil {
		t.Fatalf("service not opened; state %+v", h.m.state[path])
	}
	v := h.view()
	for _, want := range []string{"READ-ONLY", "io.vlx.demo.Greeter", "GetGreeting", "READ", "demo service"} {
		if !strings.Contains(v, want) {
			t.Errorf("view lacks %q:\n%s", want, v)
		}
	}
	// The first method is selected and described.
	if r, _ := h.m.selectedRow(); r.method == nil || r.method.Name != "GetGreeting" {
		t.Fatalf("selected %+v", r)
	}
	if !strings.Contains(v, "Who to greet.") {
		t.Errorf("detail lacks parameter docs:\n%s", v)
	}
	h.screenshot("browse")

	// Filter the tree.
	h.keys("/", "S", "u", "p", "p", "l", "i", "e", "r", "enter")
	if r, _ := h.m.selectedRow(); r.name() != "GetSupplier" {
		t.Errorf("filter selected %q", r.name())
	}
	h.keys("esc")
	if h.m.memFilter.Value() != "" {
		t.Error("esc did not clear filter")
	}

	// IDL view.
	h.selectMethod("ListItems")
	h.keys("i")
	if !strings.Contains(h.view(), "interface io.vlx.demo.Inventory") {
		t.Errorf("IDL view:\n%s", h.view())
	}
	h.keys("i")

	// Help.
	h.keys("?")
	if !strings.Contains(h.view(), "Symbols") {
		t.Error("help not shown")
	}
	h.screenshot("help")
	h.keys("esc")
	if h.m.screen != screenBrowse {
		t.Error("esc did not leave help")
	}
}

func TestCallReadMethod(t *testing.T) {
	h, _ := newHarness(t, false)
	h.selectMethod("GetGreeting")
	h.keys("enter")
	if h.m.screen != screenCall {
		t.Fatal("not on call screen")
	}
	c := h.m.call
	if c.validErr != nil {
		t.Fatalf("skeleton invalid: %v", c.validErr)
	}
	c.editor.SetValue(`{"name": "Ada", "language": "spanish"}`)
	c.validate()
	h.keys("ctrl+s")
	if c.err != nil || c.total != 1 {
		t.Fatalf("call: total %d err %v", c.total, c.err)
	}
	if !strings.Contains(h.view(), "¡Hola, Ada!") {
		t.Errorf("reply not shown:\n%s", h.view())
	}
	h.screenshot("call")

	// Errors are shown with their IDL docs.
	c.editor.SetValue(`{"name": ""}`)
	c.validate()
	h.keys("ctrl+s")
	if !strings.Contains(h.view(), "io.vlx.demo.Greeter.EmptyName") || !strings.Contains(h.view(), "The name was empty.") {
		t.Errorf("error not shown:\n%s", h.view())
	}

	// Invalid parameters are not sent.
	c.editor.SetValue(`{"name": 1}`)
	c.validate()
	sent := c.id
	h.keys("ctrl+s")
	if c.id != sent {
		t.Error("invalid parameters were sent")
	}
	h.keys("esc")
	if h.m.screen != screenBrowse || h.m.drafts["io.vlx.demo.Greeter.GetGreeting"] != `{"name": 1}` {
		t.Error("esc did not return with a saved draft")
	}
}

func TestStreamAndRedaction(t *testing.T) {
	h, _ := newHarness(t, false)
	h.selectMethod("WatchCountdown")
	h.keys("c")
	c := h.m.call
	if !c.more {
		t.Fatal("more not forced for a streaming method")
	}
	c.editor.SetValue(`{"from": 3, "intervalMilliseconds": 50}`)
	c.validate()
	h.keys("ctrl+s")
	if c.total != 4 || c.err != nil {
		t.Fatalf("stream: total %d err %v", c.total, c.err)
	}
	h.screenshot("stream")
	h.keys("esc", "esc")

	h.selectMethod("GetSupplier")
	h.keys("c")
	c = h.m.call
	c.editor.SetValue(`{"id": 1}`)
	c.validate()
	h.keys("ctrl+s")
	v := h.view()
	if strings.Contains(v, "demo-13556") || !strings.Contains(v, "«redacted»") {
		t.Errorf("token not redacted:\n%s", v)
	}
	h.keys("R")
	if !strings.Contains(h.view(), "demo-13556") {
		t.Errorf("R did not reveal:\n%s", h.view())
	}
}

func TestReadOnlyRefusesWrites(t *testing.T) {
	h, _ := newHarness(t, false)
	h.selectMethod("SetDefaultLanguage")
	h.keys("enter", "ctrl+s")
	c := h.m.call
	if c.sent || c.refused == "" || c.confirm != confirmNone {
		t.Fatalf("write was not refused: %+v", c)
	}
	if !strings.Contains(h.view(), "Not sent") {
		t.Errorf("refusal not shown:\n%s", h.view())
	}
	h.screenshot("refused")
}

func TestWritesNeedConfirmation(t *testing.T) {
	h, _ := newHarness(t, true)
	h.selectMethod("SetDefaultLanguage")
	h.keys("enter")
	c := h.m.call
	h.keys("ctrl+s")
	if c.confirm != confirmYes {
		t.Fatal("no confirmation for WRITE")
	}
	if !strings.Contains(h.view(), "Send this WRITE call?") {
		t.Errorf("dialog not shown:\n%s", h.view())
	}
	h.keys("n")
	if c.sent {
		t.Fatal("sent after n")
	}
	h.keys("ctrl+s", "y")
	if !c.sent || c.err != nil {
		t.Fatalf("not sent after y: %v", c.err)
	}
	h.keys("esc")

	h.selectMethod("RemoveItem")
	h.keys("enter")
	c = h.m.call
	c.editor.SetValue(`{"id": 4}`)
	c.validate()
	h.keys("ctrl+s")
	if c.confirm != confirmTyped {
		t.Fatal("no typed confirmation for DANGER")
	}
	h.screenshot("danger")
	h.keys("R", "e", "m", "enter")
	if c.sent {
		t.Fatal("sent with wrong confirmation")
	}
	c.confirmInput.SetValue("RemoveItem")
	h.keys("enter")
	if !c.sent || c.err != nil {
		t.Fatalf("not sent after typed confirmation: %v", c.err)
	}
}

func TestOnewayIsWrite(t *testing.T) {
	h, _ := newHarness(t, false)
	h.selectMethod("ListLanguages")
	h.keys("enter", "ctrl+o", "ctrl+s")
	if h.m.call.refused == "" {
		t.Fatal("oneway READ call was not treated as a write")
	}
}

func TestSmallTerminal(t *testing.T) {
	h, _ := newHarness(t, false)
	h.send(tea.WindowSizeMsg{Width: 50, Height: 10})
	if !strings.Contains(h.view(), "at least 60×16") {
		t.Errorf("got %s", h.view())
	}
	for _, size := range [][2]int{{80, 24}, {140, 40}, {99, 30}, {200, 60}} {
		h.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		checkFits(t, h.view(), size[0], size[1])
		h.selectMethod("GetItem")
		h.keys("enter")
		checkFits(t, h.view(), size[0], size[1])
		h.keys("esc")
	}
}

func checkFits(t *testing.T, v string, width, height int) {
	t.Helper()
	for i, line := range strings.Split(v, "\n") {
		if w := ansi.StringWidth(line); w > width {
			t.Errorf("line %d is %d wide: %q", i, w, line)
		}
	}
	if n := len(strings.Split(v, "\n")); n > height {
		t.Errorf("view is %d lines tall", n)
	}
}

func TestEscapeSequencesAreStripped(t *testing.T) {
	h, _ := newHarness(t, false)
	h.selectMethod("GetGreeting")
	h.keys("enter")
	c := h.m.call
	c.editor.SetValue("{\"name\": \"\\u001b]0;pwned\\u0007\"}")
	c.validate()
	h.keys("ctrl+s")
	raw := h.m.View()
	if strings.Contains(raw, "\x1b]0;") || strings.Contains(raw, "\a") {
		t.Fatal("OSC sequence from service data reached the terminal")
	}
}
