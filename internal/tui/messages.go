package tui

import (
	"context"
	"path/filepath"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/termenv"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
)

type discoveredMsg struct{ sockets []discovery.Socket }

type introspectedMsg struct {
	path string
	svc  *client.Service
	err  error
}

// replyMsg and callDoneMsg carry the channel they came from so that the
// model keeps draining a call it no longer shows, which lets the call's
// goroutine finish.
type replyMsg struct {
	id    int
	ch    chan tea.Msg
	reply client.Reply
}

type callDoneMsg struct {
	id  int
	err error
}

type flashMsg struct {
	text string
	err  bool
}

type flashClearMsg struct{ id int }

func discoverCmd(opts discovery.Options) tea.Cmd {
	return func() tea.Msg {
		return discoveredMsg{sockets: discovery.Discover(opts)}
	}
}

func introspectCmd(path string, opts client.Options) tea.Cmd {
	return func() tea.Msg {
		svc, err := client.Introspect(context.Background(), path, opts)
		return introspectedMsg{path: path, svc: svc, err: err}
	}
}

// startCall runs a call in the background. Replies arrive as replyMsg and
// the end as callDoneMsg; cancel stops it.
func startCall(id int, path string, req client.Request, opts client.Options) (tea.Cmd, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 64)
	go func() {
		defer close(ch)
		err := client.Call(ctx, path, req, opts, func(r client.Reply) error {
			select {
			case ch <- replyMsg{id: id, ch: ch, reply: r}:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
		ch <- callDoneMsg{id: id, err: err}
	}()
	return waitFor(ch), cancel
}

func waitFor(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// clipboard writes text to the system clipboard. It uses OSC 52, which
// works over SSH in most terminals; tests replace it.
var clipboard = termenv.Copy

func copyCmd(text, what string) tea.Cmd {
	return func() tea.Msg {
		clipboard(text)
		return flashMsg{text: "Copied " + what + " to the clipboard"}
	}
}

func clearFlashAfter(id int, d time.Duration) tea.Cmd {
	return tea.Tick(d, func(time.Time) tea.Msg { return flashClearMsg{id: id} })
}

// socketFor returns the discovered socket at path, or a synthetic entry for
// a path given on the command line.
func socketFor(path string, sockets []discovery.Socket) (discovery.Socket, bool) {
	for _, s := range sockets {
		if s.Path == path {
			return s, true
		}
	}
	return discovery.Socket{Path: path, Name: filepath.Base(path), Accessible: true}, false
}
