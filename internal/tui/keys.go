package tui

import "github.com/charmbracelet/bubbles/key"

type browseKeys struct {
	Up, Down, Left, Right, Next, Prev key.Binding
	Open, Toggle, Call, IDL, Filter   key.Binding
	Refresh, Copy, Help, Quit, Back   key.Binding
	Palette                           key.Binding
	PageUp, PageDown, Top, Bottom     key.Binding
}

var bk = browseKeys{
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Left:     key.NewBinding(key.WithKeys("left", "h"), key.WithHelp("←/h", "pane left")),
	Right:    key.NewBinding(key.WithKeys("right", "l"), key.WithHelp("→/l", "pane right")),
	Next:     key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "next pane")),
	Prev:     key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("⇧⇥", "prev pane")),
	Open:     key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "open")),
	Toggle:   key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "expand")),
	Call:     key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "call")),
	IDL:      key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "IDL")),
	Filter:   key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Refresh:  key.NewBinding(key.WithKeys("r", "ctrl+r"), key.WithHelp("r", "refresh")),
	Copy:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy cmd")),
	Help:     key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Palette:  key.NewBinding(key.WithKeys("ctrl+k", ":"), key.WithHelp("ctrl+k", "search")),
	Quit:     key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
	Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	PageUp:   key.NewBinding(key.WithKeys("pgup", "ctrl+b"), key.WithHelp("pgup", "page up")),
	PageDown: key.NewBinding(key.WithKeys("pgdown", "ctrl+f"), key.WithHelp("pgdn", "page down")),
	Top:      key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
}

type callKeys struct {
	Send, Reset, More, Oneway, Switch, Back, Mode key.Binding
	CopyCmd, CopyResp, Reveal, Quit, Help         key.Binding
	Up, Down, Top, Bottom                         key.Binding
}

var ck = callKeys{
	Send:     key.NewBinding(key.WithKeys("ctrl+s"), key.WithHelp("ctrl+s", "send")),
	Reset:    key.NewBinding(key.WithKeys("ctrl+r"), key.WithHelp("ctrl+r", "reset params")),
	More:     key.NewBinding(key.WithKeys("ctrl+t"), key.WithHelp("ctrl+t", "toggle more")),
	Oneway:   key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "toggle oneway")),
	Switch:   key.NewBinding(key.WithKeys("tab"), key.WithHelp("⇥", "editor/response")),
	Mode:     key.NewBinding(key.WithKeys("ctrl+j"), key.WithHelp("ctrl+j", "form/JSON")),
	Back:     key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "stop/back")),
	CopyCmd:  key.NewBinding(key.WithKeys("ctrl+y"), key.WithHelp("ctrl+y", "copy cmd")),
	CopyResp: key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "copy reply")),
	Reveal:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "reveal redacted")),
	Quit:     key.NewBinding(key.WithKeys("ctrl+c"), key.WithHelp("ctrl+c", "quit")),
	Help:     key.NewBinding(key.WithKeys("f1"), key.WithHelp("f1", "help")),
	Up:       key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "scroll")),
	Down:     key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "scroll")),
	Top:      key.NewBinding(key.WithKeys("home", "g"), key.WithHelp("g", "top")),
	Bottom:   key.NewBinding(key.WithKeys("end", "G"), key.WithHelp("G", "bottom")),
}
