package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
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

// call returns the method page's call, failing if there is none.
func (h *harness) call() *callState {
	h.t.Helper()
	c := h.m.currentCall()
	if c == nil {
		h.t.Fatalf("not on a method page:\n%s", h.view())
	}
	return c
}

// run types a command line and presses enter.
func (h *harness) line(s string) {
	h.t.Helper()
	if !h.m.cmd.focused {
		h.keys("ctrl+k")
	}
	h.m.cmd.input.SetValue("")
	h.typ(s)
	h.keys("enter")
}

func TestHomeGroupsAndPreview(t *testing.T) {
	h, _ := newBrowseHarness(t, true)
	h.contains("Services", "2 Varlink services on atlas", "DEMO", "OTHER", "io.vlx.demo", "a greeter and a tiny inventory", "READ-ONLY")
	h.screenshot("home")
	// The first service is previewed as soon as the list appears.
	h.contains("10 methods")
	if strings.Count(h.view(), "10 methods") != 1 {
		t.Fatal("previewed more than the selected service")
	}
	h.keys("down")
	if strings.Count(h.view(), "10 methods") != 2 {
		t.Fatalf("moving did not preview the next service:\n%s", h.view())
	}
	h.keys("home", "end", "pgup", "pgdown", "up")
	if it, _ := h.m.selected(h.m.page()); it.sock.Name != "io.vlx.demo" {
		t.Fatalf("selected %q", it.sock.Name)
	}
	// A stale preview is ignored.
	h.send(previewMsg{seq: -1, path: "/nowhere"})
}

func TestNoPreview(t *testing.T) {
	h, _ := newBrowseHarness(t, false)
	h.keys("down")
	h.lacks("methods")
	h.keys("ctrl+k")
	h.typ("get")
	h.contains("No matches", "preview is off")
	// A full line still works: it introspects the service first.
	h.line("demo GetGreeting name=Ada")
	h.contains("Hello, Ada!")
}

func TestDrillDownWithKeys(t *testing.T) {
	h, _ := newBrowseHarness(t, true, "io.vlx.demo")
	h.keys("enter")
	h.contains("Services › io.vlx.demo", "▾ io.vlx.demo.Greeter", "GetGreeting", "Returns a greeting for someone.", "▸ org.varlink.service")
	h.screenshot("service")
	if it, _ := h.m.selected(h.m.page()); it.kind != itemMethod || it.method.Name != "GetGreeting" {
		t.Fatalf("first method not selected: %+v", it)
	}
	// Collapse and expand the interface.
	h.keys("up", " ")
	h.lacks("GetGreeting")
	h.keys(" ")
	h.contains("GetGreeting")
	// Space on a method does nothing; enter opens it.
	h.keys("down", " ", "enter")
	h.contains("Services › io.vlx.demo › GetGreeting", "Parameters", "name", "Who to greet.", "Nothing sent yet", "vlx call demo GetGreeting")
	if v := h.m.cmd.input.Value(); v != "demo GetGreeting name=''" {
		t.Fatalf("command line = %q", v)
	}
	h.typ("Ada")
	h.screenshot("form")
	if v := h.m.cmd.input.Value(); v != "demo GetGreeting name=Ada" {
		t.Fatalf("command line does not mirror the form: %q", v)
	}
	h.keys("ctrl+s")
	h.contains("Hello, Ada!", "✓ 1 reply")
	h.screenshot("method")
	h.keys("esc")
	if h.m.page().kind != pageService {
		t.Fatal("esc did not go up to the service")
	}
	h.keys("left")
	if h.m.page().kind != pageHome {
		t.Fatal("left did not go up to home")
	}
	h.contains("RECENT CALLS", "GetGreeting", "name=Ada", "1 reply")
	h.keys("esc") // nothing above home
	// The recent call reopens with its arguments, unsent.
	h.keys("home", "enter")
	c := h.call()
	if v := c.form.args(); len(v) != 1 || v[0].Value != "Ada" {
		t.Fatalf("args not restored: %+v", v)
	}
}

func TestCommandLineRunsCalls(t *testing.T) {
	h, _ := newBrowseHarness(t, true, "io.vlx.demo")
	h.line("demo GetGreeting name=Ada language=german")
	h.contains("Hallo, Ada!")
	c := h.call()
	if !h.m.cmd.focused || h.m.cmd.input.Value() != "demo GetGreeting name=Ada language=german" {
		t.Fatalf("command line after a run: focused=%v %q", h.m.cmd.focused, h.m.cmd.input.Value())
	}
	// Editing the line edits the form, and ⏎ sends again.
	h.keys("backspace", "backspace", "backspace", "backspace", "backspace", "backspace")
	h.typ("spa")
	if ff := c.form.fields[1]; ff.choices[ff.choice] != "spanish" {
		t.Fatalf("form did not follow the line: %s", ff.choices[ff.choice])
	}
	h.keys("enter")
	h.contains("¡Hola, Ada!", "EARLIER ON THIS PAGE", "language=german")
	// A bad value is reported and leaves the form alone.
	h.typ(" nope=1")
	h.contains("unknown parameter \"nope\"")
	h.keys("esc")
	if h.m.cmd.focused || h.m.cmd.input.Value() != "demo GetGreeting name=Ada language=spanish" {
		t.Fatalf("esc did not restore the line: %q", h.m.cmd.input.Value())
	}
	// ↑ brings back earlier lines.
	h.keys("tab")
	h.keys("up", "up")
	if v := h.m.cmd.input.Value(); v != "demo GetGreeting name=Ada language=german" {
		t.Fatalf("recall = %q", v)
	}
	h.keys("down", "down", "down")
	h.keys("esc")

	// A method with no parameters runs straight away; so does a bare method
	// name or an interface name in place of the service.
	h.line("ListLanguages")
	h.contains("english", "japanese")
	h.line("greeter GetGreeting name=Bo")
	h.contains("Hello, Bo!")
	h.line("io.vlx.demo.Greeter.GetGreeting name=Cy")
	h.contains("Hello, Cy!")
}

func TestCommandLineSuggestions(t *testing.T) {
	h, _ := newBrowseHarness(t, true, "io.vlx.demo", "com.example.Beta")
	h.keys("down") // index the second service too
	h.typ("watch")
	h.contains("matches for watch", "WatchCountdown", "from int")
	if !h.m.cmd.focused {
		t.Fatal("typing did not focus the command line")
	}
	h.screenshot("search")
	// ⇥ completes; the popup then lists the parameters.
	h.keys("tab")
	if v := h.m.cmd.input.Value(); !strings.HasSuffix(v, " WatchCountdown ") {
		t.Fatalf("completed to %q", v)
	}
	h.contains("from=not set", "⏎ opens the method page")
	h.keys("enter")
	if c := h.call(); c.sent {
		t.Fatal("opening a method with parameters sent it")
	}
	h.keys("esc", "esc")

	// Picking a match with ↓ and ⏎ jumps to it.
	h.typ("dimensions")
	h.keys("down", "up", "down", "up", "enter")
	h.contains("type Dimensions", "Fields", "width")
	h.keys("ctrl+d")
	h.contains("interface io.vlx.demo.Inventory", "method GetItem")
	h.keys("esc", "esc", "esc")

	// The top match stands in for a line that names nothing exactly.
	h.typ("nosuchi")
	h.keys("enter")
	h.contains("error NoSuchItem", "No item has this id.")
	h.keys("esc", "esc")
	h.typ("beta")
	h.keys("enter")
	h.contains("Services › com.example.Beta")
	h.keys("esc")
	h.typ("io.vlx.demo.Inventory")
	h.keys("enter")
	h.contains("io.vlx.demo.Inventory")
	h.keys("esc")

	// Nothing matches.
	h.typ("zzzz")
	h.contains("No matches")
	h.keys("enter")
	h.contains("no socket named")
	h.keys("esc")
	h.lacks("No matches")
}

func TestCommandLineErrors(t *testing.T) {
	h, path := newHarness(t, false)
	h.line("demo Nope")
	h.contains("has no method \"Nope\"", "GetGreeting")
	h.line("demo GetGreeting name='unterminated")
	h.contains("unterminated quote")
	h.line("demo GetGreeting bogus=1")
	h.contains("unknown parameter \"bogus\"")
	// The method page opened anyway, so the user can fix it.
	h.call()
	h.line("demo")
	if h.m.page().kind != pageService || h.m.page().path != path {
		t.Fatal("a service name alone did not open the service")
	}
	h.line(path + " GetGreeting name=Ada")
	h.contains("Hello, Ada!")
	h.line("   ")
	h.line("/nowhere/at/all Describe")
	h.contains("does not exist")
}

func TestReadOnlyRefusesWrites(t *testing.T) {
	h, _ := newHarness(t, false)
	h.line("demo SetDefaultLanguage language=german")
	c := h.call()
	if c.sent || c.refused == "" || c.confirm != confirmNone {
		t.Fatalf("write was not refused: sent=%v refused=%q", c.sent, c.refused)
	}
	h.contains("⊘ not sent", "--allow-writes", "⊘ read-only")
	h.screenshot("refused")
	// Oneway turns a READ method into a write, so it is refused too.
	h.line("demo GetGreeting name=Ada")
	h.keys("esc", "ctrl+o", "ctrl+s")
	if c := h.call(); !c.oneway || c.sent || c.refused == "" {
		t.Fatal("oneway READ call was not refused")
	}
}

func TestWritesNeedConfirmation(t *testing.T) {
	h, _ := newHarness(t, true)
	h.contains("WRITES ENABLED")
	h.line("demo SetDefaultLanguage language=german")
	c := h.call()
	if c.confirm != confirmYes {
		t.Fatal("no confirmation for WRITE")
	}
	h.contains("Send this WRITE call?", "y send")
	h.screenshot("confirm")
	h.keys("x", "n")
	if c.sent || c.confirm != confirmNone {
		t.Fatal("n did not cancel")
	}
	h.contains("Cancelled; nothing was sent.")
	h.keys("ctrl+s", "y")
	if !c.sent || c.err != nil {
		t.Fatalf("y did not send: %v", c.err)
	}

	h.line("demo Reset")
	c = h.call()
	if c.confirm != confirmTyped {
		t.Fatal("no typed confirmation for DANGER")
	}
	h.contains("DANGER", "Type Reset to confirm")
	h.screenshot("danger")
	h.typ("reset")
	h.keys("enter")
	if c.sent {
		t.Fatal("wrong name sent the call")
	}
	h.contains("does not match")
	h.keys("esc")
	if c.confirm != confirmNone {
		t.Fatal("esc did not cancel")
	}
	h.keys("ctrl+s")
	h.typ("Reset")
	h.keys("enter")
	if !c.sent || c.err != nil {
		t.Fatalf("typed confirmation did not send: %v", c.err)
	}

	// Oneway sends without waiting for a reply.
	h.line("demo GetGreeting name=Ada")
	h.keys("esc", "ctrl+o", "ctrl+s", "y")
	h.contains("✓ sent", "services send no reply")
	h.keys("ctrl+o")
	if c := h.call(); c.oneway {
		t.Fatal("ctrl+o did not toggle back")
	}
}

func TestStreamStopAndRedaction(t *testing.T) {
	h, _ := newHarness(t, false)
	h.line("demo WatchCountdown from=3 intervalMilliseconds=50")
	c := h.call()
	if !c.more || c.total != 4 || c.err != nil {
		t.Fatalf("stream: more %v total %d err %v", c.more, c.total, c.err)
	}
	h.contains("✓ 4 replies", "continues")
	h.screenshot("stream")
	h.keys("esc", "ctrl+t")
	h.contains("requires 'more'")

	// esc stops a running stream; the history says so.
	h.keys("esc")
	h.m.cmd.input.SetValue("demo WatchCountdown from=100 intervalMilliseconds=50")
	h.m.cmd.focus()
	_, cmd := h.m.Update(keyFor("enter"))
	if !c.running {
		t.Fatal("stream did not start")
	}
	h.m.cmd.blur()
	h.keys("esc")
	h.run(cmd)
	if c.running || !c.stopped || c.err != nil {
		t.Fatalf("esc did not stop: running %v stopped %v err %v", c.running, c.stopped, c.err)
	}
	h.contains("■ stopped")
	if e := h.m.history[len(h.m.history)-1]; !strings.HasPrefix(e.status, "stopped after") {
		t.Fatalf("history status %q", e.status)
	}

	h.line("demo GetSupplier id=1")
	if v := h.view(); strings.Contains(v, "demo-13556") || !strings.Contains(v, "«redacted»") {
		t.Errorf("token not redacted:\n%s", v)
	}
	clip := captureClipboard(t)
	h.keys("esc", "shift+tab")
	if h.m.page().focus != focusResult {
		t.Fatal("shift+tab did not focus the result")
	}
	h.contains("↑↓ scroll", "y copy reply")
	h.keys("R")
	h.contains("demo-13556", "unredacted")
	h.keys("y", "home", "end", "pgup", "pgdown", "down")
	if len(*clip) != 1 || !strings.Contains((*clip)[0], "demo-13556") {
		t.Fatalf("clipboard %q", *clip)
	}
	h.keys("R", "esc")
	if h.m.page().focus != focusForm {
		t.Fatal("esc in the result did not return to the form")
	}
}

func TestMethodPageKeys(t *testing.T) {
	clip := captureClipboard(t)
	h, _ := newHarness(t, false)
	h.line("demo GetItem")
	c := h.call()
	h.typ("2")
	h.keys("ctrl+y")
	if len(*clip) != 1 || !strings.Contains((*clip)[0], "vlx call demo GetItem id=2") || !strings.Contains((*clip)[0], "varlinkctl call") {
		t.Fatalf("clipboard %q", *clip)
	}
	h.keys("ctrl+t")
	h.contains("does not support 'more'")

	// JSON mode and back.
	h.keys("ctrl+j")
	if !c.jsonMode {
		t.Fatal("ctrl+j did not switch to JSON")
	}
	h.contains("Parameters · JSON", "ctrl+j back to the form")
	h.keys("end")
	h.typ(" ")
	if h.m.cmd.input.Value() != "demo GetItem id=2" {
		t.Fatalf("JSON edit lost the line: %q", h.m.cmd.input.Value())
	}
	c.editor.SetValue(`{"id": "x"}`)
	h.keys(" ")
	h.contains("✗")
	h.keys("ctrl+s")
	h.contains("Fix the parameters first")
	c.editor.SetValue(`{"nope": 1}`)
	h.keys("ctrl+j")
	h.contains("the form cannot show these parameters")
	c.editor.SetValue(`{"id": 3}`)
	h.keys("ctrl+j")
	if c.jsonMode || c.form.fields[0].input.Value() != "3" {
		t.Fatal("JSON did not go back to the form")
	}
	// The line drives JSON mode too.
	h.keys("ctrl+j", "tab")
	h.m.cmd.input.SetValue("")
	h.typ("demo GetItem id=4")
	if !strings.Contains(c.editor.Value(), "4") {
		t.Fatalf("editor = %q", c.editor.Value())
	}
	h.typ(" x=1")
	h.contains("unknown parameter \"x\"")
	h.keys("shift+tab", "ctrl+j")

	h.keys("ctrl+r")
	if c.form.fields[0].input.Value() != "0" {
		t.Fatalf("ctrl+r did not reset: %q", c.form.fields[0].input.Value())
	}
	h.keys("ctrl+d")
	h.contains("method GetItem", "Returns", "Safety")
	h.keys("ctrl+d")
	h.contains("method GetItem(id: int) -> (item: Item)")
	h.keys("esc")
	h.call()
	h.keys("ctrl+s", "pgup", "pgdown")
	h.keys("f1")
	h.contains("Help", "Command line", "Method page")
	h.keys("f1")
	h.call()

	// A failed call shows the error, its documentation and the history.
	h.line("demo GetItem id=99")
	h.contains("✗ failed", "io.vlx.demo.Inventory.NoSuchItem", "No item has this id.")
	if e := h.m.history[len(h.m.history)-1]; e.ok || e.status != "NoSuchItem" {
		t.Fatalf("history %+v", e)
	}
	h.keys("esc", "esc", "esc")
	h.contains("NoSuchItem")
}

func TestServicePageKeys(t *testing.T) {
	clip := captureClipboard(t)
	h, path := newHarness(t, false)
	h.keys("ctrl+y")
	h.keys("up", "ctrl+d")
	h.contains("interface io.vlx.demo.Greeter", "IDL", "method GetGreeting(")
	h.keys("ctrl+d")
	h.contains("4 methods · 1 type · 1 error", "Press ctrl+d for the IDL source.")
	h.keys("home", "end", "pgdown", "esc")
	h.keys("right")
	h.contains("method GetGreeting(")
	h.keys("?") // ? leaves a detail page too
	if h.m.page().kind != pageService {
		t.Fatal("? did not leave the detail page")
	}
	for range 5 {
		h.keys("down")
	}
	h.keys("ctrl+y", "enter")
	h.contains("type Language", "english")
	h.keys("esc", "down", "ctrl+d")
	h.contains("error EmptyName", "Parameters")
	h.keys("esc", "end", "ctrl+y")
	want := []string{"vlx call demo GetGreeting", "io.vlx.demo.Greeter.Language", "org.varlink.service"}
	if len(*clip) != len(want) {
		t.Fatalf("clipboard %q", *clip)
	}
	for i, w := range want {
		if !strings.Contains((*clip)[i], w) {
			t.Errorf("copy %d = %q, want %q", i, (*clip)[i], w)
		}
	}
	h.keys("ctrl+r")
	h.contains("GetGreeting")
	h.keys("esc", "ctrl+y")
	if (*clip)[len(*clip)-1] != path {
		t.Fatalf("home copy = %q", (*clip)[len(*clip)-1])
	}
	h.line("demo GetGreeting name=Ada")
	h.keys("esc", "esc", "esc", "home", "ctrl+y")
	if c := (*clip)[len(*clip)-1]; c != "vlx call demo GetGreeting name=Ada" {
		t.Fatalf("recent copy = %q", c)
	}
	// Typing on a detail page goes to the command line.
	h.keys("down")
	h.keys("enter", "up", "ctrl+d")
	h.typ("x")
	if !h.m.cmd.focused || h.m.cmd.input.Value() != "x" {
		t.Fatal("typing on a detail page did not start the command line")
	}
}

func TestHelpAndQuit(t *testing.T) {
	h, _ := newHarness(t, false)
	h.keys("?")
	h.contains("Help", "SERVICE [METHOD] [NAME=VALUE …]", "READ", "DANGER", "read-only")
	h.screenshot("help")
	h.keys("down", "home", "end", "?")
	if h.m.page().kind != pageService {
		t.Fatal("? did not close help")
	}
	h.keys("f1")
	if _, ok := h.m.updateReader(h.m.page(), keyFor("q"))().(tea.QuitMsg); !ok {
		t.Fatal("q on help did not quit")
	}
	h.keys("esc")
	if _, ok := h.m.handleKey(keyFor("q"))().(tea.QuitMsg); !ok {
		t.Fatal("q did not quit")
	}
	_, cmd := h.m.Update(keyFor("ctrl+c"))
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit")
	}
	// A mouse or other message reaches the focused input harmlessly.
	h.send(struct{}{})
	h.keys("ctrl+k")
	h.send(struct{}{})
}

func TestWelcomeAndErrors(t *testing.T) {
	cfg := testConfig(t)
	h := start(t, Options{Config: cfg, Discovery: discovery0()}, 120, 36)
	h.contains("No Varlink sockets were found", "vlx demo")
	h.keys("down", "enter", "ctrl+y", "ctrl+d")
	h.m.discovered = false
	h.contains("Looking for Varlink sockets")

	// A target that cannot be opened shows why on its page.
	h = start(t, Options{Config: cfg, Discovery: discovery0(), Target: "/nonexistent.sock"}, 120, 36)
	h.contains("✗", "ctrl+r tries again")
	h.keys("ctrl+r")
	h.contains("✗")
	h.keys("esc")
	h.contains("/nonexistent.sock", "could not connect")

	// Sockets the user may not open.
	h.m.sockets[0].Accessible = false
	h.m.root = false
	h.contains("needs root")
	h.m.root = true
	h.contains("no access")
	h.keys("enter")
	h.contains("You do not have permission")
	h.m.stack = append(h.m.stack, newPage(pageService, "/nonexistent.sock"))
	delete(h.m.state, "/nonexistent.sock")
	h.contains("Try sudo")
}

func TestSmallTerminal(t *testing.T) {
	h, _ := newHarness(t, false)
	h.send(tea.WindowSizeMsg{Width: 40, Height: 10})
	h.contains("vlx needs at least 60×16")
	for _, size := range [][2]int{{60, 16}, {80, 24}, {200, 60}} {
		h.send(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		checkFits(t, h.m.View(), size[0], size[1])
		h.line("demo WatchCountdown from=2 intervalMilliseconds=50")
		checkFits(t, h.m.View(), size[0], size[1])
		h.line("demo GetItem id=1")
		h.line("demo GetItem id=2")
		h.line("demo GetItem id=3")
		h.line("demo GetItem id=4")
		checkFits(t, h.m.View(), size[0], size[1])
		h.keys("esc", "ctrl+j")
		checkFits(t, h.m.View(), size[0], size[1])
		h.keys("ctrl+j", "ctrl+d")
		checkFits(t, h.m.View(), size[0], size[1])
		h.keys("esc", "esc")
		checkFits(t, h.m.View(), size[0], size[1])
		h.keys("esc")
		checkFits(t, h.m.View(), size[0], size[1])
		h.keys("f1")
		checkFits(t, h.m.View(), size[0], size[1])
		h.keys("esc")
		h.typ("get")
		checkFits(t, h.m.View(), size[0], size[1])
		h.keys("esc")
	}
}

func TestEscapeSequencesAreStripped(t *testing.T) {
	h, _ := newHarness(t, false)
	h.line("demo GetGreeting")
	c := h.call()
	c.setParams("{\"name\": \"\\u001b]0;pwned\\u0007\"}")
	h.keys("ctrl+s")
	if c.total != 1 {
		t.Fatalf("not sent: %v", c.err)
	}
	for _, raw := range []string{h.m.View(), h.m.cmd.input.Value()} {
		if strings.Contains(raw, "\x1b]0;") || strings.Contains(raw, "\a") {
			t.Fatal("OSC sequence from service data reached the terminal")
		}
	}
}

// TestHeroScreenshot is the compact method page used on the docs home
// page: a call with earlier calls under it, at a typical terminal size.
func TestHeroScreenshot(t *testing.T) {
	h, _ := newHarness(t, false)
	h.send(tea.WindowSizeMsg{Width: 104, Height: 30})
	h.line("demo GetGreeting name=Grace")
	h.line("demo GetGreeting name=Ada language=german")
	h.line("demo GetGreeting name=Ada language=japanese")
	h.keys("esc")
	h.contains("EARLIER ON THIS PAGE", "language=german")
	checkFits(t, h.m.View(), 104, 30)
	h.screenshot("hero")
}

// TestTourScreenshots are the compact screenshots on the docs home page.
// The home page uses socket names a systemd machine has, with preview off
// so that it shows only what vlx knows without asking: each socket's group
// and description from the catalog.
func TestTourScreenshots(t *testing.T) {
	const w, ht = 112, 30
	h, _ := newBrowseHarness(t, false,
		"io.systemd.Hostname", "io.systemd.Manager", "io.systemd.Resolve", "io.systemd.Resolve.Monitor",
		"io.systemd.Network", "io.systemd.Multiplexer", "io.systemd.Login", "io.systemd.Machine",
		"io.systemd.MachineImage", "io.systemd.Credentials", "io.systemd.Udev", "io.vlx.demo")
	h.send(tea.WindowSizeMsg{Width: w, Height: ht})
	h.contains("SYSTEM", "NETWORK", "USERS & LOGIN", "DNS lookups and resolver settings")
	checkFits(t, h.m.View(), w, ht)
	h.screenshot("tour-home")

	h, _ = newHarness(t, false)
	h.send(tea.WindowSizeMsg{Width: w, Height: ht})
	h.keys("down")
	checkFits(t, h.m.View(), w, ht)
	h.screenshot("tour-service")
	h.line("demo GetItem id=2")
	h.keys("esc")
	h.contains("Unix socket (assorted)")
	checkFits(t, h.m.View(), w, ht)
	h.screenshot("tour-method")
	h.keys("esc", "esc")
	h.typ("item")
	h.contains("matches for item")
	checkFits(t, h.m.View(), w, ht)
	h.screenshot("tour-search")
}
