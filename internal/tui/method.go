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
	"github.com/rossedman/vlx/internal/cmdline"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/safety"
	"github.com/rossedman/vlx/internal/sanitize"
	"github.com/rossedman/vlx/internal/schema"
)

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

// callState is everything on one method's page: the parameters being
// edited and the replies of the last call. It outlives the page, so
// coming back to a method finds it as it was left.
type callState struct {
	path      string
	iface     *client.Interface
	method    *idl.Method
	qualified string
	skeleton  string

	form     *paramForm
	jsonMode bool
	editor   textarea.Model
	validErr error
	more     bool
	oneway   bool

	resp    viewport.Model
	replies []renderedReply
	total   int
	dropped int

	id      int
	running bool
	sent    bool
	cancel  context.CancelFunc
	started time.Time
	elapsed time.Duration
	err     error
	stopped bool
	refused string
	reveal  bool

	confirm      confirmKind
	confirmInput textinput.Model
}

func newCallState(path string, iface *client.Interface, mt *idl.Method) *callState {
	skel, _ := schema.Skeleton(iface.Parsed, mt.In)
	c := &callState{
		path:      path,
		iface:     iface,
		method:    mt,
		qualified: iface.Name + "." + mt.Name,
		skeleton:  skel,
		more:      mt.More != idl.MoreNone,
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
	c.editor = ed
	c.form = newParamForm(iface.Parsed, mt.In)
	c.setParams(skel)
	ti := textinput.New()
	ti.Prompt = "› "
	ti.PromptStyle = errStyle
	ti.Placeholder = "method name"
	ti.CharLimit = 128
	staticCursorInTests(&ti.Cursor)
	c.confirmInput = ti
	return c
}

// params returns the parameters as JSON, from the form or the JSON editor.
func (c *callState) params() string {
	if c.jsonMode {
		return c.editor.Value()
	}
	return c.form.JSON()
}

// args returns the parameters as command-line arguments.
func (c *callState) args() ([]cmdline.Arg, error) {
	if !c.jsonMode {
		return c.form.args(), nil
	}
	p, err := client.Compact([]byte(c.editor.Value()))
	if err != nil {
		return nil, err
	}
	return cmdline.Args(c.iface.Parsed, c.method.In, p)
}

// setParams loads JSON parameters into the editor and, when it can
// represent them, the form; otherwise it switches to JSON mode.
func (c *callState) setParams(s string) {
	c.editor.SetValue(s)
	if !c.jsonMode {
		if err := c.form.load(s); err != nil {
			c.jsonMode = true
		}
	}
	c.validate()
}

// setArgs loads command-line arguments into the form.
func (c *callState) setArgs(args []cmdline.Arg) error {
	if c.jsonMode {
		p, err := cmdline.Params(c.iface.Parsed, c.method.In, args)
		if err != nil {
			return err
		}
		c.editor.SetValue(string(p))
		c.validate()
		return nil
	}
	if err := c.form.loadArgs(args); err != nil {
		return err
	}
	c.validate()
	return nil
}

func (c *callState) validate() {
	if c.jsonMode {
		c.validErr = schema.Validate(c.iface.Parsed, c.method.In, []byte(c.editor.Value()))
		return
	}
	c.validErr = c.form.validate(c.method.In)
}

func (c *callState) focusEditor() tea.Cmd {
	if c.jsonMode {
		return c.editor.Focus()
	}
	return c.form.focus()
}

func (c *callState) blurEditor() {
	c.editor.Blur()
	c.form.blur()
}

// toggleMode switches between the form and the JSON editor, carrying the
// parameters across.
func (c *callState) toggleMode() error {
	if c.jsonMode {
		if err := c.form.load(c.editor.Value()); err != nil {
			return fmt.Errorf("the form cannot show these parameters: %w", err)
		}
		c.jsonMode = false
	} else {
		for _, ff := range c.form.fields {
			if _, _, problem := ff.value(); problem != "" {
				return fmt.Errorf("fix %s first: %s", ff.name, problem)
			}
		}
		c.editor.SetValue(c.form.JSON())
		c.jsonMode = true
	}
	c.blurEditor()
	c.validate()
	return nil
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
	case c.jsonMode:
		c.editor, cmd = c.editor.Update(msg)
	default:
		cmd = c.form.forward(msg)
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

// methodLayout is how the method page divides its height.
type methodLayout struct {
	doc     []string
	formH   int // lines inside the parameters box; 0 for no box
	respH   int
	earlier []historyEntry
}

func (m *Model) methodLayout(p *page) methodLayout {
	c := p.call
	w, h := m.bodyWidth(), m.bodyHeight()
	var l methodLayout
	if doc := strings.TrimSpace(c.method.Doc); doc != "" {
		l.doc = strings.Split(wrap(sanitize.Text(doc), w), "\n")
		if len(l.doc) > 3 {
			l.doc = append(l.doc[:2], ansi.Truncate(l.doc[2], w-14, "…")+" ctrl+d more")
		}
	}
	l.earlier = m.earlier(c)
	limit := max(h*45/100-2, 3)
	switch {
	case c.jsonMode:
		l.formH = min(strings.Count(c.editor.Value(), "\n")+2, limit)
	case len(c.form.fields) > 0:
		l.formH = min(c.form.lines(w-4, p.focus == focusForm && !m.cmd.focused), limit)
	}
	fixed := 1 + len(l.doc) + 1 + 1 + 1 + 1 // title, doc, blank, flags, blank, result header
	if l.formH > 0 {
		fixed += l.formH + 2
	} else {
		fixed++
	}
	if len(l.earlier) > 0 {
		fixed += 2 + len(l.earlier)
	}
	l.respH = h - fixed
	for l.respH < 3 && len(l.earlier) > 0 {
		l.earlier = l.earlier[:len(l.earlier)-1]
		l.respH++
		if len(l.earlier) == 0 {
			l.respH += 2
		}
	}
	for l.respH < 3 && len(l.doc) > 0 {
		l.doc = l.doc[:len(l.doc)-1]
		l.respH++
	}
	if l.respH < 3 && l.formH > 3 {
		cut := min(3-l.respH, l.formH-3)
		l.formH -= cut
		l.respH += cut
	}
	l.respH = max(l.respH, 1)
	return l
}

// earlier lists the other calls of this method, newest first.
func (m *Model) earlier(c *callState) []historyEntry {
	var out []historyEntry
	for i := len(m.history) - 1; i >= 0 && len(out) < 3; i-- {
		e := m.history[i]
		if e.path == c.path && e.qualified == c.qualified && e.id != c.id {
			out = append(out, e)
		}
	}
	return out
}

func (m *Model) layoutMethod(p *page) {
	c := p.call
	w := m.bodyWidth()
	l := m.methodLayout(p)
	c.editor.SetWidth(max(w-4, 10))
	c.editor.SetHeight(max(l.formH, 1))
	c.form.width = max(w-4, 10)
	c.form.height = max(l.formH, 1)
	if c.resp.Width != w || c.resp.Height != l.respH {
		c.resp.Width, c.resp.Height = w, l.respH
		m.rerenderReplies(c)
	}
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

func (m *Model) rerenderReplies(c *callState) {
	for i := range c.replies {
		c.replies[i].text = m.renderReply(c.replies[i].n, c.replies[i].reply, c.reveal)
	}
	m.refreshResponse(c, false)
}

// refreshResponse rebuilds the result. follow keeps it scrolled to the
// bottom if it was there.
func (m *Model) refreshResponse(c *callState, follow bool) {
	atBottom := c.resp.AtBottom()
	width := c.resp.Width
	var b strings.Builder
	switch {
	case c.refused != "":
		b.WriteString(wrap(textStyle.Render(c.refused), width) + "\n\n")
		b.WriteString(wrap(mutedStyle.Render("vlx starts read-only so that browsing can never change your system. To send WRITE and DANGER methods, quit and run vlx with --allow-writes (or set allow_writes: true in the config). vlx will still ask before each one."), width))
	case !c.sent:
		vlx, ctl := m.commands(c)
		b.WriteString(mutedStyle.Render("Nothing sent yet. The same call from a shell (ctrl+y copies):") + "\n")
		b.WriteString(wrap(textStyle.Render(sanitize.Line(vlx)), width) + "\n")
		b.WriteString(wrap(mutedStyle.Render(sanitize.Line(ctl)), width) + "\n")
	default:
		if c.dropped > 0 {
			b.WriteString(faintStyle.Render(fmt.Sprintf("… %d earlier replies dropped (max_replies %d)", c.dropped, m.cfg.MaxReplies)) + "\n")
		}
		for i, r := range c.replies {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(r.text + "\n")
		}
		if c.err != nil {
			b.WriteString(m.renderCallError(c, width))
		}
		if c.oneway && !c.running && c.err == nil {
			b.WriteString(mutedStyle.Render("Oneway call sent; services send no reply to oneway calls."))
		}
		if c.running && c.total == 0 {
			b.WriteString(mutedStyle.Render("Waiting for the first reply…"))
		}
	}
	c.resp.SetContent(strings.TrimRight(b.String(), "\n"))
	if follow && (atBottom || c.running) {
		c.resp.GotoBottom()
	}
}

// commands returns the call as a vlx command line and as varlinkctl.
func (m *Model) commands(c *callState) (vlx, varlinkctl string) {
	vlx, varlinkctl = render.CallCommands(c.path, c.qualified, compactOr(c.params()), c.more, c.oneway)
	if args, err := c.args(); err == nil {
		var flags []string
		if c.more {
			flags = append(flags, "--more")
		}
		if c.oneway {
			flags = append(flags, "--oneway")
		}
		vlx = strings.Join(append(append([]string{"vlx", "call"}, flags...), m.callLine(c, args)), " ")
	}
	return vlx, varlinkctl
}

// callLine is the command line for a call with args.
func (m *Model) callLine(c *callState, args []cmdline.Arg) string {
	return cmdline.Format(m.svcToken(c.path), c.method.Name, args)
}

func compactOr(s string) string {
	if p, err := client.Compact([]byte(s)); err == nil {
		return string(p)
	}
	return strings.Join(strings.Fields(s), " ")
}

func (m *Model) callStatus(c *callState) string {
	replies := fmt.Sprintf("%d %s", c.total, plural(c.total, "reply", "replies"))
	elapsed := roundDuration(c.elapsed)
	switch {
	case c.refused != "":
		return warnStyle.Bold(true).Render("⊘ not sent") + mutedStyle.Render(" · read-only mode")
	case !c.sent:
		return mutedStyle.Render("not sent yet")
	case c.running:
		return m.spinner.View() + " " + brandStyle.Render("running") + mutedStyle.Render(" · "+replies+" · "+time.Since(c.started).Round(100*time.Millisecond).String()+" · esc stops")
	case c.err != nil:
		return errStyle.Render("✗ failed") + mutedStyle.Render(" · "+elapsed)
	case c.stopped:
		return warnStyle.Render("■ stopped") + mutedStyle.Render(" · "+replies+" · "+elapsed)
	case c.oneway:
		return okStyle.Render("✓ sent") + mutedStyle.Render(" · oneway")
	default:
		return okStyle.Render("✓ "+replies) + mutedStyle.Render(" · "+elapsed)
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

func (m *Model) renderCallError(c *callState, width int) string {
	var b strings.Builder
	err := c.err
	if d, ok := client.AsVarlinkError(err); ok {
		b.WriteString(errStyle.Render("✗ "+sanitize.Line(d.Name)) + "\n")
		if len(d.Parameters) > 0 && string(d.Parameters) != "{}" {
			b.WriteString(render.JSON(d.Parameters, render.JSONOptions{Color: true, Redact: m.redactor}) + "\n")
		}
		if doc := errorDoc(m.cache[c.path], d.Name); doc != "" {
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

func (m *Model) updateMethod(p *page, msg tea.KeyMsg) tea.Cmd {
	c := p.call
	switch {
	case key.Matches(msg, mk.Send):
		return m.trySend(c)
	case key.Matches(msg, mk.Back):
		if c.running {
			c.stopped = true
			c.cancel()
			return nil
		}
		if p.focus == focusResult {
			p.focus = focusForm
			return c.focusEditor()
		}
		return m.pop()
	case key.Matches(msg, mk.Next):
		if p.focus == focusResult {
			p.focus = focusForm
			return c.focusEditor()
		}
		c.blurEditor()
		return m.cmd.focus()
	case key.Matches(msg, mk.Prev):
		if p.focus == focusForm {
			p.focus = focusResult
			c.blurEditor()
			return nil
		}
		return m.cmd.focus()
	case key.Matches(msg, gk.Command):
		c.blurEditor()
		return m.cmd.focus()
	case key.Matches(msg, mk.Mode):
		if err := c.toggleMode(); err != nil {
			return m.setFlash(err.Error(), false)
		}
		p.focus = focusForm
		m.layoutMethod(p)
		return c.focusEditor()
	case key.Matches(msg, mk.More):
		switch c.method.More {
		case idl.MoreRequired:
			return m.setFlash("This method requires 'more'; it always streams.", false)
		case idl.MoreNone:
			return m.setFlash("This method does not support 'more'.", false)
		}
		c.more = !c.more
		if c.more {
			c.oneway = false
		}
		m.refreshResponse(c, false)
		return nil
	case key.Matches(msg, mk.Oneway):
		c.oneway = !c.oneway
		if c.oneway {
			c.more = false
		} else {
			c.more = c.method.More != idl.MoreNone
		}
		m.refreshResponse(c, false)
		return nil
	case key.Matches(msg, mk.Reset):
		c.form = newParamForm(c.iface.Parsed, c.method.In)
		c.jsonMode = false
		c.setParams(c.skeleton)
		m.layoutMethod(p)
		m.refreshResponse(c, false)
		m.syncCommand()
		if p.focus == focusResult {
			c.blurEditor()
			return nil
		}
		return c.focusEditor()
	case key.Matches(msg, mk.Copy):
		vlx, ctl := m.commands(c)
		return copyCmd(vlx+"\n"+ctl, "the vlx and varlinkctl commands")
	case key.Matches(msg, mk.Docs):
		c.blurEditor()
		m.openDetail(c.path, item{kind: itemMethod, iface: c.iface, method: c.method}, false)
		return nil
	case key.Matches(msg, mk.PageUp):
		c.resp.PageUp()
		return nil
	case key.Matches(msg, mk.PageDown):
		c.resp.PageDown()
		return nil
	}

	if p.focus == focusResult {
		switch {
		case key.Matches(msg, mk.CopyReply):
			return copyCmd(responseJSON(c), "the reply JSON")
		case key.Matches(msg, mk.Reveal):
			c.reveal = !c.reveal
			m.rerenderReplies(c)
			if c.reveal {
				return m.setFlash("Redacted values revealed for this result", true)
			}
			return nil
		case key.Matches(msg, lk.Top):
			c.resp.GotoTop()
			return nil
		case key.Matches(msg, lk.Bottom):
			c.resp.GotoBottom()
			return nil
		case msg.String() == "?":
			return m.openHelp()
		}
		var cmd tea.Cmd
		c.resp, cmd = c.resp.Update(msg)
		return cmd
	}

	if !c.jsonMode {
		cmd, changed := c.form.update(msg)
		if changed {
			m.paramsChanged(c)
		}
		return cmd
	}
	before := c.editor.Value()
	var cmd tea.Cmd
	c.editor, cmd = c.editor.Update(msg)
	if c.editor.Value() != before {
		m.paramsChanged(c)
	}
	return cmd
}

func (m *Model) paramsChanged(c *callState) {
	c.validate()
	m.refreshResponse(c, false)
	m.syncCommand()
}

// responseJSON returns the kept replies as newline-delimited compact JSON,
// unredacted (the clipboard is the user's own).
func responseJSON(c *callState) string {
	var lines []string
	for _, r := range c.replies {
		lines = append(lines, string(r.reply.Params))
	}
	return strings.Join(lines, "\n")
}

func (m *Model) trySend(c *callState) tea.Cmd {
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
		m.refreshResponse(c, false)
		return m.setFlash("Refused: read-only mode", false)
	}
	c.refused = ""
	switch d.Confirm {
	case safety.ConfirmYes:
		c.confirm = confirmYes
		c.blurEditor()
		m.cmd.blur()
		return nil
	case safety.ConfirmTyped:
		c.confirm = confirmTyped
		c.blurEditor()
		m.cmd.blur()
		c.confirmInput.SetValue("")
		return c.confirmInput.Focus()
	}
	return m.send(c)
}

func (m *Model) updateConfirm(c *callState, msg tea.KeyMsg) tea.Cmd {
	cancel := func() tea.Cmd {
		c.confirm = confirmNone
		c.confirmInput.Blur()
		return tea.Batch(m.refocus(), m.setFlash("Cancelled; nothing was sent.", true))
	}
	switch c.confirm {
	case confirmYes:
		switch msg.String() {
		case "y", "Y":
			c.confirm = confirmNone
			return tea.Batch(m.send(c), m.refocus())
		case "n", "N", "esc", "q":
			return cancel()
		}
		return nil
	default:
		switch msg.String() {
		case "esc":
			return cancel()
		case "enter":
			if c.confirmInput.Value() == c.method.Name {
				c.confirm = confirmNone
				c.confirmInput.Blur()
				return tea.Batch(m.send(c), m.refocus())
			}
			return m.setFlash("That does not match; type "+c.method.Name+" exactly, or esc to cancel.", false)
		}
		var cmd tea.Cmd
		c.confirmInput, cmd = c.confirmInput.Update(msg)
		return cmd
	}
}

// refocus gives the keyboard back to the method page after a dialog.
func (m *Model) refocus() tea.Cmd {
	p := m.page()
	if p.kind != pageMethod || p.focus != focusForm {
		return nil
	}
	return p.call.focusEditor()
}

func (m *Model) send(c *callState) tea.Cmd {
	params, err := client.Compact([]byte(c.params()))
	if err != nil {
		return m.setFlash("Invalid JSON: "+err.Error(), false)
	}
	m.callID++
	c.id = m.callID
	c.replies, c.total, c.dropped = nil, 0, 0
	c.err, c.stopped, c.reveal = nil, false, false
	c.running, c.sent = true, true
	c.started = time.Now()
	m.record(c)
	req := client.Request{Method: c.qualified, Params: params, More: c.more, Oneway: c.oneway}
	cmd, cancel := startCall(c.id, c.path, req, m.copts)
	c.cancel = cancel
	if p := m.page(); p.kind == pageMethod && p.call == c {
		m.layoutMethod(p)
	}
	m.refreshResponse(c, true)
	return tea.Batch(cmd, m.spinner.Tick)
}

// record adds a call to the history as it is sent.
func (m *Model) record(c *callState) {
	h := historyEntry{id: c.id, path: c.path, qualified: c.qualified, method: c.method.Name, at: c.started, status: "running"}
	if args, err := c.args(); err == nil {
		h.line = m.callLine(c, args)
		h.args = cmdline.Format("", "", args)
	} else {
		h.line = m.callLine(c, nil)
		h.args = compactOr(c.params())
	}
	if h.args == "" {
		h.args = "no parameters"
	}
	m.history = append(m.history, h)
	if len(m.history) > 200 {
		m.history = m.history[len(m.history)-200:]
	}
	m.cmd.remember(h.line)
}

// recordDone fills in how a call ended.
func (m *Model) recordDone(c *callState) {
	for i := len(m.history) - 1; i >= 0; i-- {
		h := &m.history[i]
		if h.id != c.id {
			continue
		}
		h.done, h.ok = true, c.err == nil
		switch {
		case c.err != nil:
			h.status = c.err.Error()
			if d, ok := client.AsVarlinkError(c.err); ok {
				h.status = d.Name[strings.LastIndexByte(d.Name, '.')+1:]
			}
		case c.stopped:
			h.status = fmt.Sprintf("stopped after %d %s", c.total, plural(c.total, "reply", "replies"))
		case c.oneway:
			h.status = "sent oneway"
		default:
			h.status = fmt.Sprintf("%d %s", c.total, plural(c.total, "reply", "replies"))
		}
		return
	}
}

func (m *Model) viewMethod(p *page, width, height int) string {
	c := p.call
	m.layoutMethod(p)
	l := m.methodLayout(p)
	v := c.verdict(m.classifier)
	formFocused := p.focus == focusForm && !m.cmd.focused

	streams := "single reply"
	switch c.method.More {
	case idl.MoreRequired:
		streams = "streams"
	case idl.MoreSupported:
		streams = "can stream"
	}
	lines := []string{
		boldStyle.Render(sanitize.Line(c.method.Name)) + "  " + classTag(v.Class) + mutedStyle.Render("  · "+sanitize.Line(c.iface.Name)+" · "+streams),
	}
	for _, d := range l.doc {
		lines = append(lines, docStyle.Render(d))
	}
	lines = append(lines, "")

	switch {
	case l.formH > 0:
		title := "Parameters"
		var body string
		if c.jsonMode {
			title = "Parameters · JSON"
			body = c.editor.View()
		} else {
			body = c.form.view(width-4, l.formH, formFocused)
		}
		lines = append(lines, framed(title, strings.Split(body, "\n"), width, l.formH, formFocused)...)
	default:
		lines = append(lines, mutedStyle.Render("No parameters.")+faintStyle.Render(" ctrl+s sends the call."))
	}

	flags := toggle("more", c.more, c.method.More != idl.MoreNone) + "  " + toggle("oneway", c.oneway, true) + "   "
	if c.validErr != nil {
		flags += errStyle.Render("✗ " + sanitize.Line(c.validErr.Error()))
	} else {
		flags += okStyle.Render("✓ valid")
	}
	if d := m.policy.Decide(c.qualified, v); !d.Allowed {
		flags += "  " + warnStyle.Render("⊘ read-only: will not be sent")
	}
	mode := faintStyle.Render("ctrl+j edit as JSON")
	if c.jsonMode {
		mode = faintStyle.Render("ctrl+j back to the form")
	}
	if len(c.form.fields) == 0 {
		mode = ""
	}
	lines = append(lines, columns(width, flags, "", mode), "")

	rtitle := sectionTitle("Result", p.focus == focusResult && !m.cmd.focused) + "  " + m.callStatus(c)
	if c.resp.TotalLineCount() > c.resp.Height {
		rtitle += faintStyle.Render(fmt.Sprintf("  %d%%", int(c.resp.ScrollPercent()*100)))
	}
	if c.reveal {
		rtitle += "  " + warnStyle.Render("unredacted")
	}
	lines = append(lines, rtitle)
	resp := strings.Split(c.resp.View(), "\n")
	for len(resp) < l.respH {
		resp = append(resp, "")
	}
	lines = append(lines, resp[:l.respH]...)

	if len(l.earlier) > 0 {
		lines = append(lines, "", sectionTitle("Earlier on this page", false)+faintStyle.Render("  ↑ on the command line brings one back"))
		for _, e := range l.earlier {
			lines = append(lines, m.earlierLine(e, width))
		}
	}
	return strings.Join(lines, "\n")
}

func (m *Model) earlierLine(e historyEntry, width int) string {
	mark := m.spinner.View()
	switch {
	case e.done && e.ok:
		mark = okStyle.Render("✓")
	case e.done:
		mark = errStyle.Render("✗")
	}
	return columns(width, mark+" "+textStyle.Render(sanitize.Line(e.args)), mutedStyle.Render(sanitize.Line(e.status)), faintStyle.Render(e.at.Format("15:04:05")))
}

func sectionTitle(s string, focused bool) string {
	st := brandStyle
	if focused {
		st = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF")).Background(render.BrandDeep).Bold(true)
		return st.Render(" " + strings.ToUpper(s) + " ")
	}
	return st.Render(strings.ToUpper(s))
}

// framed draws lines in a rounded box of the given outer width, with the
// title set into the top border.
func framed(title string, lines []string, width, height int, focused bool) []string {
	bc := faintStyle
	tc := mutedStyle
	if focused {
		bc = brandStyle
		tc = brandStyle
	}
	inner := max(width-4, 1)
	t := " " + title + " "
	top := bc.Render("╭─") + tc.Render(t) + bc.Render(strings.Repeat("─", max(width-3-ansi.StringWidth(t), 0))+"╮")
	out := []string{top}
	for i := 0; i < height; i++ {
		l := ""
		if i < len(lines) {
			l = ansi.Truncate(lines[i], inner, "…")
		}
		out = append(out, bc.Render("│")+" "+padRight(l, inner)+" "+bc.Render("│"))
	}
	return append(out, bc.Render("╰"+strings.Repeat("─", max(width-2, 0))+"╯"))
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

func (m *Model) viewConfirm(c *callState) string {
	v := c.verdict(m.classifier)
	color := classColor(v.Class)
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Foreground(color).Bold(true).Render(fmt.Sprintf("Send this %s call?", v.Class)) + "\n\n")
	b.WriteString(mutedStyle.Render("method   ") + boldStyle.Render(sanitize.Line(c.qualified)) + "\n")
	b.WriteString(mutedStyle.Render("socket   ") + textStyle.Render(sanitize.Line(c.path)) + "\n")
	b.WriteString(mutedStyle.Render("reason   ") + textStyle.Render(sanitize.Line(v.Reason)) + "\n")
	if m.root {
		b.WriteString(mutedStyle.Render("as       ") + errStyle.Render("root") + "\n")
	}
	params := compactOr(c.params())
	if len(params) > 200 {
		params = params[:200] + "…"
	}
	b.WriteString(mutedStyle.Render("params   ") + render.StringStyle.Render(sanitize.Line(params)) + "\n\n")
	switch c.confirm {
	case confirmYes:
		b.WriteString(textStyle.Render("This changes state on the service. ") + brandStyle.Render("y") + mutedStyle.Render(" send · ") + brandStyle.Render("n") + mutedStyle.Render(" cancel"))
	case confirmTyped:
		b.WriteString(errStyle.Render("This is a DANGER method. It may be disruptive or hard to undo.") + "\n")
		b.WriteString(textStyle.Render("Type ") + boldStyle.Render(sanitize.Line(c.method.Name)) + textStyle.Render(" to confirm:") + "\n\n")
		b.WriteString(c.confirmInput.View())
	}
	w := min(m.bodyWidth()-4, 76)
	return dialogStyle.BorderForeground(color).Width(w).Render(b.String())
}
