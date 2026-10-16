package tui

import (
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
)

func TestMatchScoreRanking(t *testing.T) {
	entries := []entry{
		{kind: entryMethod, service: "io.systemd.Resolve", iface: "io.systemd.Resolve", name: "ResolveHostname"},
		{kind: entryMethod, service: "io.systemd.Resolve", iface: "io.systemd.Resolve", name: "ResolveAddress"},
		{kind: entryMethod, service: "io.systemd.Hostname", iface: "io.systemd.Hostname", name: "Describe"},
		{kind: entryService, service: "io.systemd.Hostname", name: "Hostname"},
		{kind: entryType, service: "io.systemd.Resolve", iface: "io.systemd.Resolve", name: "ResolvedAddress"},
	}
	best := func(q string) string {
		top, topScore := "", -1<<30
		for _, e := range entries {
			if s, ok := matchScore(q, e); ok && s > topScore {
				top, topScore = e.qualified(), s
			}
		}
		return top
	}
	for q, want := range map[string]string{
		"resolve host":  "io.systemd.Resolve.ResolveHostname",
		"hostname":      "io.systemd.Hostname",
		"hostname desc": "io.systemd.Hostname.Describe",
		"rsvhst":        "io.systemd.Resolve.ResolveHostname", // subsequence of the name
		"addr":          "io.systemd.Resolve.ResolveAddress",
	} {
		if got := best(q); got != want {
			t.Errorf("best(%q) = %s, want %s", q, got, want)
		}
	}
	if _, ok := matchScore("zzz", entries[0]); ok {
		t.Error("nonsense matched")
	}
}

func TestCatalog(t *testing.T) {
	xdg := t.TempDir()
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	for _, tc := range []struct {
		path, name, group, desc, label string
	}{
		{"/run/systemd/resolve/io.systemd.Resolve", "io.systemd.Resolve", groupNetwork, "DNS lookups and resolver settings", "Resolve"},
		{"/run/systemd/userdb/io.systemd.Home", "io.systemd.Home", groupUsers, "User and group records from Home", "Home (userdb)"},
		{"/run/systemd/userdb/io.systemd.Multiplexer", "io.systemd.Multiplexer", groupUsers, "Every user and group, from all user databases", "Multiplexer"},
		{"/run/systemd/report/io.systemd.Network", "io.systemd.Network", groupSystem, "Metrics from Network", "Network (metrics)"},
		{filepath.Join(xdg, "systemd/io.systemd.Manager"), "io.systemd.Manager", groupSession, "The service manager (PID 1): units and jobs (yours)", "Manager"},
		{filepath.Join(xdg, "systemd/io.example.Thing"), "io.example.Thing", groupSession, "A service of your user manager", "io.example.Thing"},
		{"/run/other/com.example.X", "com.example.X", groupOther, "", "com.example.X"},
	} {
		s := discovery.Socket{Path: tc.path, Name: tc.name}
		g, d := describe(s)
		if g != tc.group || d != tc.desc || label(s) != tc.label {
			t.Errorf("%s: %q %q %q", tc.path, g, d, label(s))
		}
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
	for d, want := range map[time.Duration]string{1500 * time.Millisecond: "1.5s", 10 * time.Microsecond: "<1ms", 12 * time.Millisecond: "12ms"} {
		if got := roundDuration(d); got != want {
			t.Errorf("roundDuration(%s) = %s", d, got)
		}
	}
	if toggle("x", false, false) == "" || compactOr("{ bad") != "{ bad" {
		t.Error("toggle or compactOr")
	}
	if summary("One. Two.") != "One." || columns(10, "a very long name indeed", "middle", "R") == "" {
		t.Error("summary or columns")
	}
	if got := overlayBottom([]string{"a"}, []string{"x", "y"}); got[0] != "y" {
		t.Errorf("overlayBottom = %q", got)
	}
}

func TestCallStateDetails(t *testing.T) {
	h, _ := newHarness(t, false)
	h.line("demo ListItems")
	c := h.call()
	if c.total < 2 {
		t.Fatalf("ListItems sent %d replies", c.total)
	}
	// Only max_replies replies are kept.
	h.m.cfg.MaxReplies = 1
	h.line("demo ListItems")
	h.contains("earlier replies dropped")
	// Blinks reach the form, the JSON editor and the confirmation input.
	h.keys("esc")
	c.forward(struct{}{})
	h.keys("ctrl+j")
	c.forward(struct{}{})
	c.confirm = confirmTyped
	c.forward(struct{}{})
	c.confirm = confirmNone
	if h.m.callByID(-1) != nil {
		t.Error("callByID found a call that does not exist")
	}
	// Quitting stops a running call.
	h.m.cmd.input.SetValue("demo WatchCountdown from=100 intervalMilliseconds=50")
	h.m.cmd.focus()
	_, cmd := h.m.Update(keyFor("enter"))
	_, quit := h.m.Update(keyFor("ctrl+c"))
	if _, ok := quit().(tea.QuitMsg); !ok {
		t.Fatal("ctrl+c did not quit")
	}
	h.run(cmd)
	if c := h.call(); !c.stopped && c.err == nil {
		t.Fatalf("call still running: %+v", c.err)
	}
}
