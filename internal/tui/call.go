package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/rossedman/sdvarlink/idl"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/sanitize"
	"github.com/rossedman/vlx/internal/schema"
)

type textinputModel = textinput.Model

type confirmKind int

const (
	confirmNone confirmKind = iota
	confirmYes
	confirmTyped
)

type renderedReply struct {
	n     int
	reply client.Reply
	text  string
}

// callState is the call screen: an editor for one method's parameters and
// the replies of the last call.
type callState struct {
	row       row
	qualified string
	skeleton  string
	optional  []string

	editor    textarea.Model
	validErr  error
	more      bool
	oneway    bool
	focusResp bool

	resp    viewport.Model
	replies []renderedReply
	total   int
	dropped int

	id       int
	running  bool
	sent     bool
	cancel   context.CancelFunc
	started  time.Time
	elapsed  time.Duration
	err      error
	stopped  bool
	refused  string
	reveal   bool
	lastSent string

	confirm      confirmKind
	confirmInput textinput.Model
}

func (m *Model) openCall(r row) tea.Cmd {
	skel, optional := skeletonFor(r)
	c := &callState{
		row:       r,
		qualified: r.qualified(),
		skeleton:  skel,
		optional:  optional,
		more:      r.method.More != idl.MoreNone,
		resp:      viewport.New(40, 10),
	}
	ed := textarea.New()
	ed.ShowLineNumbers = true
	ed.Prompt = " "
	ed.CharLimit = 1 << 20
	ed.MaxHeight = 0
	ed.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(render.Surface)
	ed.FocusedStyle.LineNumber = faintStyle
	ed.FocusedStyle.CursorLineNumber = brandStyle
	ed.BlurredStyle.LineNumber = faintStyle
	ed.FocusedStyle.Text = textStyle
	ed.BlurredStyle.Text = mutedStyle
	ed.KeyMap.TransposeCharacterBackward.SetEnabled(false) // ctrl+t toggles more
	staticCursorInTests(&ed.Cursor)
	ed.SetValue(skel)
	if d, ok := m.drafts[c.qualified]; ok {
		ed.SetValue(d)
	}
	c.editor = ed
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = errStyle
	ti.Placeholder = "method name"
	ti.CharLimit = 128
	staticCursorInTests(&ti.Cursor)
	c.confirmInput = ti
	m.call = c
	m.screen = screenCall
	c.validate()
	m.layoutCall()
	m.refreshResponse(false)
	return c.editor.Focus()
}

func (c *callState) validate() {
	c.validErr = schema.Validate(c.row.iface.Parsed, c.row.method.In, []byte(c.editor.Value()))
}

func (c *callState) verdict(cl *safety.Classifier) safety.Verdict {
	return cl.ClassifyCall(c.qualified, c.oneway)
}

// forward hands non-key messages (cursor blink) to the focused input.
func (c *callState) forward(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	switch {
	case c.confirm == confirmTyped:
		c.confirmInput, cmd = c.confirmInput.Update(msg)
	case !c.focusResp:
		c.editor, cmd = c.editor.Update(msg)
	}
	return cmd
}

func (c *callState) addReply(r client.Reply, maxKeep int, renderFn func(int, client.Reply, bool) string) {
	c.total++
	c.replies = append(c.replies, renderedReply{n: c.total, reply: r, text: renderFn(c.total, r, c.reveal)})
	if over := len(c.replies) - maxKeep; over > 0 {
		c.replies = append(c.replies[:0], c.replies[over:]...)
		c.dropped += over
	}
	c.elapsed = time.Since(c.started)
}

func (c *callState) finish(err error) {
	c.running = false
	c.elapsed = time.Since(c.started)
	c.cancel = nil
	if errors.Is(err, context.Canceled) && c.stopped {
		err = nil
	}
	c.err = err
}

func (m *Model) layoutCall() {
	c := m.call
	if c == nil {
		return
	}
	h := m.bodyHeight()
	edW, respW, stacked := m.callWidths()
	edH, respH := h, h
	if stacked {
		edH = h / 2
		respH = h - edH
	}
	// Editor pane: border (2) + title + flags + blank, validation + optional
	// hints below.
	c.editor.SetWidth(max(edW-4, 10))
	c.editor.SetHeight(max(edH-2-3-m.hintLines(edW), 3))
	c.resp.Width = max(respW-4, 10)
	c.resp.Height = max(respH-2-1, 3)
	m.rerenderReplies()
}

func (m *Model) callWidths() (editor, response int, stacked bool) {
	if m.width < 100 {
		return m.width, m.width, true
	}
	editor = clamp(m.width*2/5, 40, 72)
	return editor, m.width - editor, false
}

func (m *Model) hintLines(width int) int {
	if m.call == nil {
		return 0
	}
	n := 1 // validation line
	if len(m.call.optional) > 0 {
		n += min(len(m.call.optional), 4) + 1
	}
	_ = width
	return n
}

func (m *Model) renderReply(n int, r client.Reply, reveal bool) string {
	red := m.redactor
	if reveal {
		red = nil
	}
	var b strings.Builder
	meta := fmt.Sprintf("#%d  +%s", n, roundDuration(r.Elapsed))
	if r.Continues {
		meta += "  continues"
	}
	if r.Files > 0 {
		meta += fmt.Sprintf("  %d fd(s) received and closed", r.Files)
	}
	b.WriteString(faintStyle.Render(meta) + "\n")
	b.WriteString(render.JSON(r.Params, render.JSONOptions{Color: true, Redact: red}))
	return b.String()
}

func (m *Model) rerenderReplies() {
	c := m.call
	if c == nil {
		return
	}
	for i := range c.replies {
		c.replies[i].text = m.renderReply(c.replies[i].n, c.replies[i].reply, c.reveal)
	}
	m.refreshResponse(false)
}

// refreshResponse rebuilds the response pane. follow keeps it scrolled to
// the bottom if it was there.
func (m *Model) refreshResponse(follow bool) {
	c := m.call
	if c == nil {
		return
	}
	atBottom := c.resp.AtBottom()
	width := c.resp.Width
	var b strings.Builder
	switch {
	case c.refused != "":
		b.WriteString(warnStyle.Bold(true).Render("⊘ Not sent") + "\n\n")
		b.WriteString(wrap(textStyle.Render(c.refused), width) + "\n\n")
		b.WriteString(wrap(mutedStyle.Render("vlx starts read-only so that browsing can never change your system. To send WRITE and DANGER methods, quit and run vlx with --allow-writes (or set allow_writes: true in the config). vlx will still ask before each one."), width))
	case !c.sent:
		b.WriteString(mutedStyle.Render("Edit the parameters, then press ") + brandStyle.Render("ctrl+s") + mutedStyle.Render(" to send.") + "\n\n")
		vlx, ctl := render.CallCommands(m.svcPath, c.qualified, compactOr(c.editor.Value()), c.more, c.oneway)
		b.WriteString(mutedStyle.Render("Equivalent commands") + mutedStyle.Render(" (ctrl+y copies)") + "\n")
		b.WriteString(wrap(textStyle.Render(sanitize.Line(vlx)), width) + "\n")
		b.WriteString(wrap(mutedStyle.Render(sanitize.Line(ctl)), width) + "\n")
	default:
		b.WriteString(m.callStatusLine() + "\n")
		if c.dropped > 0 {
			b.WriteString(faintStyle.Render(fmt.Sprintf("… %d earlier replies dropped (max_replies %d)", c.dropped, m.cfg.MaxReplies)) + "\n")
		}
		for _, r := range c.replies {
			b.WriteString("\n" + r.text + "\n")
		}
		if c.err != nil {
			b.WriteString("\n" + m.renderCallError(c.err, width))
		}
		if c.oneway && !c.running && c.err == nil {
			b.WriteString("\n" + mutedStyle.Render("Oneway call sent; services send no reply to oneway calls."))
		}
	}
	c.resp.SetContent(b.String())
	if follow && (atBottom || c.running) {
		c.resp.GotoBottom()
	}
}

func compactOr(s string) string {
	if p, err := client.Compact([]byte(s)); err == nil {
		return string(p)
	}
	return strings.Join(strings.Fields(s), " ")
}

func (m *Model) callStatusLine() string {
	c := m.call
	replies := fmt.Sprintf("%d %s", c.total, plural(c.total, "reply", "replies"))
	elapsed := roundDuration(c.elapsed)
	switch {
	case c.running:
		return m.spinner.View() + " " + brandStyle.Render("running") + mutedStyle.Render(" · "+replies+" · "+time.Since(c.started).Round(100*time.Millisecond).String()+" · esc stops")
	case c.err != nil:
		return errStyle.Render("✗ failed") + mutedStyle.Render(" · "+replies+" · "+elapsed)
	case c.stopped:
		return warnStyle.Render("■ stopped") + mutedStyle.Render(" · "+replies+" · "+elapsed)
	default:
		return okStyle.Render("✓ done") + mutedStyle.Render(" · "+replies+" · "+elapsed)
	}
}

func roundDuration(d time.Duration) string {
	switch {
	case d < time.Millisecond:
		return "<1ms"
	case d < time.Second:
		return d.Round(time.Millisecond).String()
	}
	return d.Round(10 * time.Millisecond).String()
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

func (m *Model) renderCallError(err error, width int) string {
	var b strings.Builder
	if d, ok := client.AsVarlinkError(err); ok {
		b.WriteString(errStyle.Render("✗ "+sanitize.Line(d.Name)) + "\n")
		if len(d.Parameters) > 0 && string(d.Parameters) != "{}" {
			b.WriteString(render.JSON(d.Parameters, render.JSONOptions{Color: true, Redact: m.redactor}) + "\n")
		}
		if doc := errorDoc(m.svc, d.Name); doc != "" {
			b.WriteString(wrap(mutedStyle.Render(sanitize.Text(doc)), width) + "\n")
		}
	} else {
		b.WriteString(wrap(errStyle.Render("✗ "+sanitize.Line(err.Error())), width) + "\n")
	}
	if hint := client.Hint(err); hint != "" {
		b.WriteString("\n" + wrap(mutedStyle.Render(hint), width) + "\n")
	}
	return b.String()
}

func (m *Model) callHelpKeys() []key.Binding {
	c := m.call
	if c == nil {
		return nil
	}
	switch c.confirm {
	case confirmYes:
		return []key.Binding{
			key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "send")),
			key.NewBinding(key.WithKeys("n"), key.WithHelp("n/esc", "cancel")),
		}
	case confirmTyped:
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("⏎", "confirm")),
			key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "cancel")),
		}
	}
	keys := []key.Binding{ck.Send, ck.Switch}
	if c.row.method.More == idl.MoreSupported {
		keys = append(keys, ck.More)
	}
	keys = append(keys, ck.Oneway, ck.Reset, ck.CopyCmd)
	if c.focusResp {
		keys = append(keys, ck.CopyResp, ck.Reveal)
	}
	stop := ck.Back
	if c.running {
		stop.SetHelp("esc", "stop")
	} else {
		stop.SetHelp("esc", "back")
	}
	return append(keys, stop)
}

func (m *Model) updateCall(msg tea.KeyMsg) tea.Cmd {
	c := m.call
	if c.confirm != confirmNone {
		return m.updateConfirm(msg)
	}
	switch {
	case key.Matches(msg, ck.Back):
		if c.running {
			c.stopped = true
			m.stopCall()
			return nil
		}
		m.drafts[c.qualified] = c.editor.Value()
		m.screen = screenBrowse
		m.focus = paneMembers
		m.layout()
		return nil
	case key.Matches(msg, ck.Help):
		m.helpReturn = screenCall
		m.screen = screenHelp
		return nil
	case key.Matches(msg, ck.Send):
		return m.trySend()
	case key.Matches(msg, ck.Switch):
		c.focusResp = !c.focusResp
		if c.focusResp {
			c.editor.Blur()
			return nil
		}
		return c.editor.Focus()
	case key.Matches(msg, ck.More):
		switch c.row.method.More {
		case idl.MoreRequired:
			return m.setFlash("This method requires 'more'; it always streams.", false)
		case idl.MoreNone:
			return m.setFlash("This method does not support 'more'.", false)
		}
		c.more = !c.more
		if c.more {
			c.oneway = false
		}
		m.refreshResponse(false)
		return nil
	case key.Matches(msg, ck.Oneway):
		c.oneway = !c.oneway
		if c.oneway {
			c.more = false
		} else {
			c.more = c.row.method.More != idl.MoreNone
		}
		m.refreshResponse(false)
		return nil
	case key.Matches(msg, ck.Reset):
		c.editor.SetValue(c.skeleton)
		c.validate()
		m.refreshResponse(false)
		return nil
	case key.Matches(msg, ck.CopyCmd):
		vlx, ctl := render.CallCommands(m.svcPath, c.qualified, compactOr(c.editor.Value()), c.more, c.oneway)
		return copyCmd(vlx+"\n"+ctl, "the vlx and varlinkctl commands")
	}

	if c.focusResp {
		switch {
		case key.Matches(msg, ck.CopyResp):
			return copyCmd(m.responseJSON(), "the reply JSON")
		case key.Matches(msg, ck.Reveal):
			c.reveal = !c.reveal
			m.rerenderReplies()
			if c.reveal {
				return m.setFlash("Redacted values revealed for this response", true)
			}
			return nil
		case key.Matches(msg, ck.Top):
			c.resp.GotoTop()
			return nil
		case key.Matches(msg, ck.Bottom):
			c.resp.GotoBottom()
			return nil
		}
		var cmd tea.Cmd
		c.resp, cmd = c.resp.Update(msg)
		return cmd
	}

	before := c.editor.Value()
	var cmd tea.Cmd
	c.editor, cmd = c.editor.Update(msg)
	if c.editor.Value() != before {
		c.validate()
		m.refreshResponse(false)
	}
	return cmd
}

// responseJSON returns the kept replies as newline-delimited compact JSON,
// unredacted (the clipboard is the user's own).
func (m *Model) responseJSON() string {
	var lines []string
	for _, r := range m.call.replies {
		lines = append(lines, string(r.reply.Params))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) trySend() tea.Cmd {
	c := m.call
	if c.running {
		return m.setFlash("A call is already running; esc stops it.", false)
	}
	if c.validErr != nil {
		return m.setFlash("Fix the parameters first: "+c.validErr.Error(), false)
	}
	v := c.verdict(m.classifier)
	d := m.policy.Decide(c.qualified, v)
	if !d.Allowed {
		c.refused = d.Why
		c.sent = false
		m.refreshResponse(false)
		return m.setFlash("Refused: read-only mode", false)
	}
	c.refused = ""
	switch d.Confirm {
	case safety.ConfirmYes:
		c.confirm = confirmYes
		c.editor.Blur()
		return nil
	case safety.ConfirmTyped:
		c.confirm = confirmTyped
		c.editor.Blur()
		c.confirmInput.SetValue("")
		return c.confirmInput.Focus()
	}
	return m.send()
}

func (m *Model) updateConfirm(msg tea.KeyMsg) tea.Cmd {
	c := m.call
	cancel := func() tea.Cmd {
		c.confirm = confirmNone
		c.confirmInput.Blur()
		if !c.focusResp {
			return tea.Batch(c.editor.Focus(), m.setFlash("Cancelled; nothing was sent.", true))
		}
		return m.setFlash("Cancelled; nothing was sent.", true)
	}
	switch c.confirm {
	case confirmYes:
		switch msg.String() {
		case "y", "Y":
			c.confirm = confirmNone
			return m.send()
		case "n", "N", "esc", "q":
			return cancel()
		}
		return nil
	case confirmTyped:
		switch msg.String() {
		case "esc":
			return cancel()
		case "enter":
			if c.confirmInput.Value() == c.row.method.Name {
				c.confirm = confirmNone
				c.confirmInput.Blur()
				return m.send()
			}
			return m.setFlash("That does not match; type "+c.row.method.Name+" exactly, or esc to cancel.", false)
		}
		var cmd tea.Cmd
		c.confirmInput, cmd = c.confirmInput.Update(msg)
		return cmd
	}
	return nil
}

func (m *Model) send() tea.Cmd {
	c := m.call
	params, err := client.Compact([]byte(c.editor.Value()))
	if err != nil {
		return m.setFlash("Invalid JSON: "+err.Error(), false)
	}
	m.callID++
	c.id = m.callID
	c.replies, c.total, c.dropped = nil, 0, 0
	c.err, c.stopped, c.reveal = nil, false, false
	c.running, c.sent = true, true
	c.started = time.Now()
	c.lastSent = string(params)
	c.focusResp = true
	c.editor.Blur()
	req := client.Request{Method: c.qualified, Params: params, More: c.more, Oneway: c.oneway}
	cmd, cancel := startCall(c.id, m.svcPath, req, m.copts)
	c.cancel = cancel
	m.refreshResponse(true)
	return tea.Batch(cmd, m.spinner.Tick)
}

func (m *Model) viewCall() string {
	c := m.call
	edW, respW, stacked := m.callWidths()
	h := m.bodyHeight()
	edH, respH := h, h
	if stacked {
		edH = h / 2
		respH = h - edH
	}

	// Editor pane.
	v := c.verdict(m.classifier)
	title := lipgloss.JoinHorizontal(lipgloss.Top,
		brandStyle.Render("⌁ "), boldStyle.Render(sanitize.Line(c.qualified)))
	if c.row.method.More == idl.MoreNone {
		title = brandStyle.Render("ƒ ") + boldStyle.Render(sanitize.Line(c.qualified))
	}
	flags := classTag(v.Class) + "  " + toggle("more", c.more, c.row.method.More != idl.MoreNone) + "  " + toggle("oneway", c.oneway, true)
	if d := m.policy.Decide(c.qualified, v); !d.Allowed {
		flags += "  " + warnStyle.Render("⊘ read-only")
	}
	var lines []string
	lines = append(lines, ansi.Truncate(flags, edW-4, "…"), "")
	lines = append(lines, strings.Split(c.editor.View(), "\n")...)
	if c.validErr != nil {
		lines = append(lines, ansi.Truncate(errStyle.Render("✗ "+sanitize.Line(c.validErr.Error())), edW-4, "…"))
	} else {
		lines = append(lines, okStyle.Render("✓ valid parameters"))
	}
	if len(c.optional) > 0 {
		lines = append(lines, mutedStyle.Render("Optional:"))
		for i, o := range c.optional {
			if i == 3 && len(c.optional) > 4 {
				lines = append(lines, faintStyle.Render(fmt.Sprintf("  … %d more (see Detail)", len(c.optional)-3)))
				break
			}
			lines = append(lines, ansi.Truncate(render.KeyStyle.Render("  "+sanitize.Line(o)), edW-4, "…"))
		}
	}
	editor := box(ansi.Truncate(title, edW-6, "…"), !c.focusResp, edW, edH, lines)

	// Response pane.
	rtitle := "Response"
	if c.resp.TotalLineCount() > c.resp.Height {
		rtitle += mutedStyle.Render(fmt.Sprintf(" %d%%", int(c.resp.ScrollPercent()*100)))
	}
	if c.reveal {
		rtitle += " " + warnStyle.Render("· unredacted")
	}
	resp := box(rtitle, c.focusResp, respW, respH, strings.Split(c.resp.View(), "\n"))
	if stacked {
		return lipgloss.JoinVertical(lipgloss.Left, editor, resp)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, editor, resp)
}

func toggle(name string, on, available bool) string {
	switch {
	case !available:
		return faintStyle.Render("○ " + name)
	case on:
		return brandStyle.Render("● " + name)
	default:
		return mutedStyle.Render("○ " + name)
	}
}

func (m *Model) viewConfirm() string {
	c := m.call
	v := c.verdict(m.classifier)
	color := classColor(v.Class)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(color).Bold(true).Render(fmt.Sprintf("Send this %s call?", v.Class)) + "\n\n")
	b.WriteString(mutedStyle.Render("method   ") + boldStyle.Render(sanitize.Line(c.qualified)) + "\n")
	b.WriteString(mutedStyle.Render("socket   ") + textStyle.Render(sanitize.Line(m.svcPath)) + "\n")
	b.WriteString(mutedStyle.Render("reason   ") + textStyle.Render(sanitize.Line(v.Reason)) + "\n")
	if m.root {
		b.WriteString(mutedStyle.Render("as       ") + errStyle.Render("root") + "\n")
	}
	params := compactOr(c.editor.Value())
	if len(params) > 200 {
		params = params[:200] + "…"
	}
	b.WriteString(mutedStyle.Render("params   ") + render.StringStyle.Render(sanitize.Line(params)) + "\n\n")
	switch c.confirm {
	case confirmYes:
		b.WriteString(textStyle.Render("This changes state on the service. ") + brandStyle.Render("y") + mutedStyle.Render(" send · ") + brandStyle.Render("n") + mutedStyle.Render(" cancel"))
	case confirmTyped:
		b.WriteString(errStyle.Render("This is a DANGER method. It may be disruptive or hard to undo.") + "\n")
		b.WriteString(textStyle.Render("Type ") + boldStyle.Render(sanitize.Line(c.row.method.Name)) + textStyle.Render(" to confirm:") + "\n\n")
		b.WriteString(c.confirmInput.View())
	}
	w := min(m.width-4, 76)
	return dialogStyle.BorderForeground(color).Width(w).Render(b.String())
}
