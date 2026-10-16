package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/x/ansi"
)

type globalKeys struct{ Command key.Binding }

var gk = globalKeys{
	Command: key.NewBinding(key.WithKeys("ctrl+k"), key.WithHelp("ctrl+k", "command line")),
}

type listKeys struct {
	Up, Down, PageUp, PageDown, Top, Bottom key.Binding
	Open, Toggle, Back, Refresh, Docs, Copy key.Binding
}

var lk = listKeys{
	Up:       key.NewBinding(key.WithKeys("up"), key.WithHelp("↑", "up")),
	Down:     key.NewBinding(key.WithKeys("down"), key.WithHelp("↓", "down")),
	PageUp:   key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "page down")),
	Top:      key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
	Bottom:   key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
	Open:     key.NewBinding(key.WithKeys("enter", "right"), key.WithHelp("⏎ →", "open")),
	Toggle:   key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expand or collapse")),
	Back:     key.NewBinding(key.WithKeys("esc", "left", "backspace"), key.WithHelp("esc ←", "up a level")),
	Refresh:  key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "refresh")),
	Docs:     key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "docs and IDL")),
	Copy:     key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp("ctrl+y", "copy")),
}

type methodKeys struct {
	Send, Back, Next, Prev, Mode, More, Oneway, Reset, Copy, Docs key.Binding
	PageUp, PageDown, CopyReply, Reveal                           key.Binding
}

var mk = methodKeys{
	Send:      key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send")),
	Back:      key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop · up a level")),
	Next:      key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "form → command line → result")),
	Prev:      key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧⇥", "the other way")),
	Mode:      key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "form or JSON")),
	More:      key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "toggle more (stream)")),
	Oneway:    key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "toggle oneway")),
	Reset:     key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "reset parameters")),
	Copy:      key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp("ctrl+y", "copy as vlx / varlinkctl")),
	Docs:      key.NewBinding(key.WithKeys("ctrl+d"), key.WithHelp("ctrl+d", "full docs")),
	PageUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll result")),
	PageDown:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "scroll result")),
	CopyReply: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy reply JSON")),
	Reveal:    key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reveal redacted")),
}

// viewHints is the row of keys under the command line.
func (m *Model) viewHints() string {
	pairs := m.hints()
	parts := make([]string, len(pairs))
	for i, p := range pairs {
		parts[i] = brandStyle.Render(p[0]) + " " + mutedStyle.Render(p[1])
	}
	return ansi.Truncate(strings.Repeat(" ", gutter)+strings.Join(parts, "   "), m.width, "…")
}

func (m *Model) hints() [][2]string {
	p := m.page()
	if c := m.currentCall(); c != nil && c.confirm != confirmNone {
		if c.confirm == confirmYes {
			return [][2]string{{"y", "send"}, {"n esc", "cancel"}}
		}
		return [][2]string{{"⏎", "confirm"}, {"esc", "cancel"}}
	}
	if m.cmd.focused {
		switch {
		case len(m.cmd.sugg) > 0:
			return [][2]string{{"↑↓", "pick"}, {"⏎", "open"}, {"⇥", "complete"}, {"esc", "back to the page"}}
		case p.kind == pageMethod:
			return [][2]string{{"⏎", "send"}, {"↑↓", "earlier calls"}, {"⇥", "result"}, {"esc", "back to the form"}}
		case m.cmd.target != nil:
			return [][2]string{{"⏎", "go"}, {"esc", "cancel"}}
		}
		return [][2]string{{"⏎", "go"}, {"↑↓", "earlier calls"}, {"esc", "cancel"}}
	}
	switch p.kind {
	case pageHome:
		return [][2]string{{"↑↓", "move"}, {"⏎", "open"}, {"a-z", "jump anywhere"}, {"ctrl+y", "copy"}, {"ctrl+r", "refresh"}, {"?", "help"}, {"q", "quit"}}
	case pageService:
		return [][2]string{{"↑↓", "move"}, {"⏎", "open"}, {"space", "expand"}, {"ctrl+d", "docs · IDL"}, {"esc", "back"}, {"a-z", "jump"}, {"?", "help"}}
	case pageMethod:
		if p.focus == focusResult {
			return [][2]string{{"↑↓", "scroll"}, {"y", "copy reply"}, {"R", "reveal"}, {"ctrl+s", "send again"}, {"⇥", "form"}, {"esc", "back to the form"}}
		}
		stop := "back"
		if p.call.running {
			stop = "stop"
		}
		return [][2]string{{"ctrl+s", "send"}, {"⇥", "command line"}, {"ctrl+j", "JSON"}, {"ctrl+y", "copy as command"}, {"ctrl+d", "docs"}, {"esc", stop}, {"f1", "help"}}
	case pageHelp:
		return [][2]string{{"↑↓", "scroll"}, {"esc", "back"}, {"q", "quit"}}
	}
	return [][2]string{{"↑↓", "scroll"}, {"ctrl+d", "docs · IDL"}, {"esc", "back"}}
}
