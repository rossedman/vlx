package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestMatchScoreRanking(t *testing.T) {
	entries := []paletteEntry{
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
		"rsvhst":        "io.systemd.Resolve.ResolveHostname", // subsequence
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

func TestPaletteJumps(t *testing.T) {
	previewDelay = time.Millisecond
	h := newBrowseHarness(t, true)
	h.send(tea.KeyMsg{Type: tea.KeyCtrlK})
	if h.m.palette == nil {
		t.Fatal("ctrl+k did not open the palette")
	}
	// Opening indexes every service, so both are searchable.
	if len(h.m.cache) != 2 {
		t.Fatalf("indexed %d services, want 2", len(h.m.cache))
	}
	if !strings.Contains(h.view(), "Search everything") {
		t.Fatalf("palette not drawn:\n%s", h.view())
	}
	h.keys("b", "e", "t", "a", " ", "c", "o", "u", "n", "t")
	h.screenshot("palette")
	res := h.m.palette.results
	if len(res) == 0 || res[0].name != "WatchCountdown" || !strings.HasSuffix(res[0].path, "com.example.Beta") {
		t.Fatalf("top result = %+v", res)
	}
	h.keys("enter")
	if h.m.palette != nil {
		t.Fatal("palette still open after enter")
	}
	r, _ := h.m.selectedRow()
	if h.m.focus != paneMembers || r.method == nil || r.method.Name != "WatchCountdown" || !strings.HasSuffix(h.m.svcPath, "com.example.Beta") {
		t.Fatalf("jumped to %s on %s (focus %v)", r.qualified(), h.m.svcPath, h.m.focus)
	}
	// Jumping into a collapsed interface expands it.
	h.send(tea.KeyMsg{Type: tea.KeyCtrlK})
	h.keys("G", "e", "t", "I", "n", "f", "o", "enter")
	r, _ = h.m.selectedRow()
	if r.method == nil || r.qualified() != "org.varlink.service.GetInfo" {
		t.Fatalf("jumped to %s", r.qualified())
	}
	// Esc closes without moving.
	h.keys(":", "x", "esc")
	if h.m.palette != nil {
		t.Fatal("esc did not close")
	}
}
