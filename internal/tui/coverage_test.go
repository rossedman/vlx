package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/viper"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/demo"
	"github.com/rossedman/vlx/internal/discovery"
)

// captureClipboard records what copy commands put on the clipboard.
func captureClipboard(t *testing.T) *[]string {
	t.Helper()
	var got []string
	old := clipboard
	clipboard = func(s string) { got = append(got, s) }
	t.Cleanup(func() { clipboard = old })
	return &got
}

func keyMsg(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// setJSON replaces the call's parameters.
func setJSON(c *callState, s string) {
	c.editor.SetValue(s)
	c.validate()
}

func selectRow(h *harness, kind rowKind, name string) {
	h.t.Helper()
	for i, r := range h.m.rows {
		if r.kind == kind && r.name() == name {
			h.m.memCursor = i
			h.m.focus = paneMembers
			h.m.refreshDetail()
			return
		}
	}
	h.t.Fatalf("row %s not found", name)
}

func TestBrowseKeys(t *testing.T) {
	clip := captureClipboard(t)
	h, path := newHarness(t, false)

	// esc walks back out to the services pane.
	h.keys("esc")
	if h.m.focus != paneServices {
		t.Fatalf("focus %v", h.m.focus)
	}
	h.keys("down", "up", "G", "g")
	h.send(keyMsg(tea.KeyPgDown))
	h.keys("enter") // already open: moves into the tree
	if h.m.focus != paneMembers {
		t.Fatal("enter on the open service did not move into the tree")
	}
	// Pane cycling in both directions.
	h.keys("tab")
	if h.m.focus != paneDetail {
		t.Fatalf("tab: %v", h.m.focus)
	}
	h.keys("down", "esc")
	if h.m.focus != paneMembers {
		t.Fatal("esc from detail")
	}
	h.send(keyMsg(tea.KeyShiftTab))
	h.keys("l", "h", "right", "left")

	// Expand and collapse an interface.
	h.m.focus = paneMembers
	h.m.memCursor = 0
	h.keys("enter")
	if h.m.expanded["io.vlx.demo.Greeter"] {
		t.Fatal("enter did not collapse")
	}
	if !strings.Contains(h.view(), "▸ io.vlx.demo.Greeter") {
		t.Error("collapsed marker not shown")
	}
	h.keys(" ")
	if !h.m.expanded["io.vlx.demo.Greeter"] {
		t.Fatal("space did not expand")
	}

	// Interface, type and error details.
	h.m.memCursor = 0
	h.m.refreshDetail()
	if !strings.Contains(h.view(), "methods ·") {
		t.Errorf("interface detail:\n%s", h.view())
	}
	selectRow(h, rowType, "Language")
	h.keys("enter")
	if h.m.focus != paneDetail || !strings.Contains(h.view(), "Values") || !strings.Contains(h.view(), "Used by") {
		t.Errorf("type detail:\n%s", h.view())
	}
	selectRow(h, rowType, "Item")
	if !strings.Contains(h.view(), "Fields") {
		t.Errorf("struct detail:\n%s", h.view())
	}
	selectRow(h, rowError, "NoSuchItem")
	if !strings.Contains(h.view(), "error NoSuchItem") {
		t.Errorf("error detail:\n%s", h.view())
	}
	h.keys("y")
	selectRow(h, rowMethod, "WatchCountdown")
	h.keys("y")
	if len(*clip) != 2 || (*clip)[0] != "io.vlx.demo.Inventory.NoSuchItem" || !strings.HasPrefix((*clip)[1], "vlx call --more "+path) {
		t.Fatalf("clipboard %q", *clip)
	}
	selectRow(h, rowMethod, "SetDefaultLanguage")
	if !strings.Contains(h.view(), "Blocked in read-only mode") {
		t.Error("read-only note missing")
	}

	// Filtering the services list.
	h.m.focus = paneServices
	h.keys("/", "z", "z")
	if !strings.Contains(h.view(), "no match") {
		t.Errorf("filter:\n%s", h.view())
	}
	h.keys("down") // leaves the filter
	if h.m.filtering {
		t.Fatal("still filtering")
	}
	h.keys("esc") // clears it
	if h.m.svcFilter.Value() != "" {
		t.Fatal("esc did not clear the services filter")
	}
	h.keys("/", "x", "esc")
	// The detail pane has no filter.
	h.m.focus = paneDetail
	h.keys("/")
	if h.m.filtering {
		t.Fatal("filtering the detail pane")
	}

	// Refresh re-introspects.
	h.keys("r")
	if st := h.m.state[path]; st == nil || !st.ok {
		t.Fatalf("refresh: %+v", st)
	}

	// Help and quitting.
	h.keys("?")
	if h.m.screen != screenHelp {
		t.Fatal("no help")
	}
	if cmd := h.m.updateHelp(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd == nil {
		t.Fatal("q in help did not quit")
	}
	_, cmd := h.m.Update(keyMsg(tea.KeyCtrlC))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit")
	}
	if _, ok := h.m.updateBrowse(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})().(tea.QuitMsg); !ok {
		t.Fatal("q did not quit")
	}
}

// newPlainModel builds a model with discovery limited to dirs.
func newPlainModel(t *testing.T, dirs []string, target string) *harness {
	t.Helper()
	v := viper.New()
	config.SetDefaults(v)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	cfg, err := config.Load(v, "")
	if err != nil {
		t.Fatal(err)
	}
	m := New(Options{Config: cfg, Discovery: discovery.Options{Dirs: dirs}, Target: target})
	h := &harness{t: t, m: m}
	h.send(tea.WindowSizeMsg{Width: 120, Height: 36})
	return h
}

func TestWelcomeScreens(t *testing.T) {
	h := newPlainModel(t, []string{}, "")
	if !strings.Contains(h.view(), "Looking for Varlink sockets") || !strings.Contains(h.view(), "searching") {
		t.Errorf("before discovery:\n%s", h.view())
	}
	h.run(h.m.Init())
	if !strings.Contains(h.view(), "No Varlink sockets were found") || !strings.Contains(h.view(), "no sockets found") {
		t.Errorf("nothing found:\n%s", h.view())
	}

	srv, err := demo.ListenTemp()
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	h = newPlainModel(t, []string{filepath.Dir(srv.Path())}, "")
	h.run(h.m.Init())
	if !strings.Contains(h.view(), "Press ⏎ to connect") {
		t.Errorf("selected:\n%s", h.view())
	}
	h.m.svcFilter.SetValue("nomatch")
	h.m.refreshDetail()
	if !strings.Contains(h.view(), "Found 1 services") {
		t.Errorf("found:\n%s", h.view())
	}
	h.m.svcFilter.SetValue("")
	// Opening shows a spinner state until the reply arrives.
	_, cmd := h.m.Update(keyMsg(tea.KeyEnter))
	if !strings.Contains(h.view(), "introspecting") || !h.m.busy() {
		t.Errorf("loading:\n%s", h.view())
	}
	h.run(cmd)
	if h.m.svc == nil {
		t.Fatal("not opened")
	}

	// A socket you may not use, and one that failed.
	h.m.focus = paneServices
	h.m.sockets = append(h.m.sockets,
		discovery.Socket{Path: "/run/x/locked", Name: "io.example.Locked"},
		discovery.Socket{Path: "/run/x/broken", Name: "io.example.Broken", Accessible: true})
	h.m.state["/run/x/broken"] = &sockState{err: &os.PathError{Op: "dial", Path: "/run/x/broken", Err: syscall.EACCES}}
	h.m.svcCursor = 1
	h.m.refreshDetail()
	if !strings.Contains(h.view(), "permission to connect") || !strings.Contains(h.view(), "⊘") {
		t.Errorf("locked:\n%s", h.view())
	}
	h.m.svcCursor = 2
	h.m.refreshDetail()
	if !strings.Contains(h.view(), "✗") || !strings.Contains(h.view(), "ermission") {
		t.Errorf("broken:\n%s", h.view())
	}
	// Opening a broken socket reports it.
	h.m.svcCursor = 1
	h.keys("enter")
	if h.m.svc != nil || h.m.focus != paneServices {
		t.Fatal("a failed open left a service selected")
	}
}

func TestCallKeys(t *testing.T) {
	clip := captureClipboard(t)
	h, _ := newHarness(t, false)

	h.selectMethod("GetGreeting")
	h.keys("enter", "ctrl+t")
	if !strings.Contains(h.m.flash, "does not support") {
		t.Errorf("flash %q", h.m.flash)
	}
	h.keys("esc")
	h.selectMethod("WatchCountdown")
	h.keys("enter", "ctrl+t")
	if !strings.Contains(h.m.flash, "requires") {
		t.Errorf("flash %q", h.m.flash)
	}
	h.keys("esc")

	h.selectMethod("ListItems")
	h.keys("enter")
	c := h.m.call
	h.keys("ctrl+t")
	if c.more {
		t.Fatal("ctrl+t did not turn more off")
	}
	h.keys("ctrl+t", "ctrl+o")
	if !c.oneway || c.more {
		t.Fatal("oneway should turn more off")
	}
	h.keys("ctrl+o")
	if c.oneway || !c.more {
		t.Fatal("oneway off should restore more")
	}
	c.editor.SetValue(`{"tag": "tools"}`)
	h.keys("ctrl+r")
	if c.editor.Value() != c.skeleton {
		t.Fatal("ctrl+r did not reset")
	}
	h.send(keyMsg(tea.KeyCtrlY))
	if len(*clip) != 1 || !strings.Contains((*clip)[0], "varlinkctl call --more") {
		t.Fatalf("clipboard %q", *clip)
	}
	h.keys("ctrl+s")
	if c.total != 4 {
		t.Fatalf("%d items", c.total)
	}
	// Response pane keys.
	if !c.focusResp {
		t.Fatal("focus did not move to the response")
	}
	h.keys("down", "up", "G", "g", "y", "R", "R")
	if len(*clip) != 2 || strings.Count((*clip)[1], "\n") != 3 {
		t.Fatalf("reply JSON %q", (*clip)[1])
	}
	h.keys("tab")
	if c.focusResp {
		t.Fatal("tab did not return to the editor")
	}
	// Typing in the editor revalidates.
	h.keys("x")
	if c.validErr == nil {
		t.Fatal("invalid JSON not flagged")
	}
	h.send(keyMsg(tea.KeyF1))
	if h.m.screen != screenHelp {
		t.Fatal("f1 did not open help")
	}
	h.keys("esc")
	if h.m.screen != screenCall {
		t.Fatal("help did not return to the call")
	}
	// Non-key messages reach the editor (cursor blinks).
	h.m.Update(struct{}{})
	if compactOr("{ bad\n json") != "{ bad json" {
		t.Error("compactOr")
	}
}

func TestStopStreamAndLimits(t *testing.T) {
	h, _ := newHarness(t, false)
	h.m.cfg.MaxReplies = 2
	h.selectMethod("WatchCountdown")
	h.keys("enter")
	c := h.m.call
	setJSON(c, `{"from": 3, "intervalMilliseconds": 50}`)
	h.keys("ctrl+s")
	if c.dropped != 2 || !strings.Contains(h.view(), "2 earlier replies dropped") {
		t.Fatalf("dropped %d:\n%s", c.dropped, h.view())
	}

	// Start a long stream without draining it, then stop it.
	setJSON(c, `{"from": 100, "intervalMilliseconds": 50}`)
	cmd := h.m.trySend()
	if !c.running || !h.m.busy() {
		t.Fatal("not running")
	}
	h.m.updateCall(keyMsg(tea.KeyCtrlS))
	if !strings.Contains(h.m.flash, "already running") {
		t.Errorf("flash %q", h.m.flash)
	}
	h.m.updateCall(keyMsg(tea.KeyEsc))
	h.run(cmd)
	if c.running || !c.stopped || c.err != nil {
		t.Fatalf("running %v stopped %v err %v", c.running, c.stopped, c.err)
	}
	if !strings.Contains(h.view(), "stopped") {
		t.Errorf("view:\n%s", h.view())
	}

	// Errors that are not Varlink errors are shown too.
	c.finish(errors.New("boom"))
	h.m.refreshResponse(false)
	if !strings.Contains(h.view(), "boom") {
		t.Errorf("view:\n%s", h.view())
	}
	c.finish(context.DeadlineExceeded)
	h.m.refreshResponse(false)
	if !strings.Contains(h.view(), "did not answer in time") {
		t.Errorf("hint missing:\n%s", h.view())
	}
}

func TestOnewayWithWrites(t *testing.T) {
	h, _ := newHarness(t, true)
	h.selectMethod("SetDefaultLanguage")
	h.keys("enter", "ctrl+o", "ctrl+s")
	c := h.m.call
	if c.confirm != confirmYes {
		t.Fatal("no confirmation")
	}
	h.m.Update(struct{}{}) // forwarded while confirming
	h.keys("y")
	if !strings.Contains(h.view(), "Oneway call sent") {
		t.Errorf("view:\n%s", h.view())
	}
	// Cancelling from the response pane.
	h.keys("ctrl+s", "esc")
	if c.confirm != confirmNone || !strings.Contains(h.m.flash, "nothing was sent") {
		t.Fatal("esc did not cancel")
	}
	// Typed confirmation: forwarding and esc.
	h.keys("esc")
	h.selectMethod("Reset")
	h.keys("enter", "ctrl+s")
	h.m.Update(struct{}{})
	h.keys("x", "esc")
	if h.m.call.sent {
		t.Fatal("sent after esc")
	}
}

func TestHelpers(t *testing.T) {
	if errorDoc(nil, "a.B") != "" {
		t.Error("nil service")
	}
	svc := &client.Service{}
	if errorDoc(svc, "nodot") != "" || errorDoc(svc, "a.B") != "" {
		t.Error("errorDoc")
	}
	off := 0
	if s, e := listWindow(0, 5, 0, &off); s != 0 || e != 0 {
		t.Error("listWindow with no height")
	}
	if usersOf(nil, "x") != nil || namedTypes(nil, nil) != nil {
		t.Error("nil interface")
	}
	if kindLabel(nil) != "Fields" {
		t.Error("kindLabel")
	}
	if roundDuration(1500*1e6) != "1.5s" {
		t.Errorf("roundDuration %s", roundDuration(1500*1e6))
	}
	if toggle("x", false, false) == "" {
		t.Error("toggle")
	}
}
