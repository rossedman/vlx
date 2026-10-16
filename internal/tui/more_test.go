package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func TestPaletteRowsAndKeys(t *testing.T) {
	h := newBrowseHarness(t, true)
	h.send(tea.KeyMsg{Type: tea.KeyCtrlK})
	// An empty query lists services.
	if n := len(h.m.palette.results); n != 2 || h.m.palette.results[0].kind != entryService {
		t.Fatalf("empty query: %d results", n)
	}
	h.keys("N", "o", "S", "u", "c", "h")
	v := h.view()
	if !strings.Contains(v, "NoSuchItem") || !strings.Contains(v, "error") {
		t.Errorf("error rows:\n%s", v)
	}
	for _, q := range []string{"Dimensions", "io.vlx.demo.Inventory", "Alpha"} {
		h.m.palette.input.SetValue(q)
		h.m.refreshPalette()
		if len(h.m.palette.results) == 0 {
			t.Fatalf("no results for %q", q)
		}
		for _, r := range h.m.palette.results[:1] {
			row := ansi.Strip(h.m.paletteRow(r, 60, false))
			if !strings.Contains(row, r.name) {
				t.Errorf("row %q lacks %q", row, r.name)
			}
		}
	}
	// Moving and narrow rows.
	h.send(tea.KeyMsg{Type: tea.KeyDown})
	h.send(tea.KeyMsg{Type: tea.KeyUp})
	if row := h.m.paletteRow(h.m.palette.results[0], 12, true); ansi.StringWidth(row) > 12 {
		t.Errorf("row too wide: %q", ansi.Strip(row))
	}
	// Jump to an interface and to a service.
	h.m.palette.input.SetValue("io.vlx.demo.Inventory")
	h.m.refreshPalette()
	h.keys("enter")
	if r, _ := h.m.selectedRow(); r.kind != rowIface || r.iface.Name != "io.vlx.demo.Inventory" {
		t.Fatalf("jumped to %s", r.qualified())
	}
	h.send(tea.KeyMsg{Type: tea.KeyCtrlK})
	h.m.palette.input.SetValue("Beta")
	h.m.refreshPalette()
	h.keys("enter")
	if !strings.HasSuffix(h.m.svcPath, "Beta") || h.m.focus != paneMembers {
		t.Fatalf("service jump: %s %v", h.m.svcPath, h.m.focus)
	}
	// No matches; blink messages reach the input while open.
	h.send(tea.KeyMsg{Type: tea.KeyCtrlK})
	h.keys("z", "z", "z", "q")
	h.m.Update(struct{}{})
	if !strings.Contains(h.view(), "No matches") {
		t.Errorf("no matches:\n%s", h.view())
	}
	h.keys("enter") // nothing to jump to
	h.send(tea.KeyMsg{Type: tea.KeyCtrlK})
	if h.m.palette != nil {
		t.Fatal("ctrl+k did not close the palette")
	}
}

func TestPaletteWithoutPreview(t *testing.T) {
	h := newBrowseHarness(t, false)
	h.send(tea.KeyMsg{Type: tea.KeyCtrlK})
	if len(h.m.cache) != 0 || !strings.Contains(h.view(), "preview is off") {
		t.Fatalf("indexed with preview off:\n%s", h.view())
	}
	// Jumping to a member of an unopened service is impossible, but a
	// service opens it.
	h.keys("B", "e", "t", "a", "enter")
	if h.m.svc == nil {
		t.Fatal("service not opened from the palette")
	}
}

func TestStalePreviewIgnored(t *testing.T) {
	h := newBrowseHarness(t, true)
	before := len(h.m.state)
	if cmd := h.m.handlePreview(previewMsg{seq: -1, path: h.m.svcPath}); cmd != nil {
		t.Error("stale preview ran")
	}
	if cmd := h.m.handlePreview(previewMsg{seq: h.m.previewSeq, path: h.m.svcPath}); cmd != nil {
		t.Error("cached service re-introspected")
	}
	if len(h.m.state) != before {
		t.Error("state changed")
	}
	if h.m.showService("") != nil {
		t.Error("empty path")
	}
}

func TestFormHintsAndForward(t *testing.T) {
	f, _ := testForm(t)
	seen := map[fieldKind]bool{}
	for _, ff := range f.fields {
		seen[ff.kind] = true
		_ = ff.hint()
	}
	if len(seen) != 6 {
		t.Fatalf("kinds %v", seen)
	}
	if field(f, "color").hint() == "" || !strings.Contains(field(f, "shade").hint(), "red") {
		t.Error("enum hints")
	}
	f.cursor = 0
	f.focus()
	f.forward(struct{}{})
	f.cursor = 3 // a choice: nothing to forward
	if f.forward(struct{}{}) != nil {
		t.Error("forwarded to a choice")
	}
	empty := newParamForm(nil, nil)
	if empty.current() != nil || empty.move(1) != nil || empty.JSON() != "{}" {
		t.Error("empty form")
	}
	if !strings.Contains(empty.view(40, 5, true), "no parameters") {
		t.Error("empty view")
	}
	if jsonPlaceholder(field(f, "tags").base) == jsonPlaceholder(field(f, "where").base) {
		t.Error("placeholders should differ for arrays and objects")
	}
	if compactJSON([]byte("{bad")) != "{bad" {
		t.Error("compactJSON invalid")
	}
}
