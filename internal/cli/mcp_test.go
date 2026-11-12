package cli

import (
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/rossedman/vlx/internal/config"
)

// heldReader serves data, then holds the stream open until done reports
// true or a deadline passes, as a client would while it waits for answers.
type heldReader struct {
	r    io.Reader
	done func() bool
}

// Read returns data, then waits before reporting EOF.
func (h *heldReader) Read(p []byte) (int, error) {
	n, err := h.r.Read(p)
	if err != io.EOF {
		return n, err
	}
	for deadline := time.Now().Add(10 * time.Second); !h.done() && time.Now().Before(deadline); {
		time.Sleep(10 * time.Millisecond)
	}
	return 0, io.EOF
}

// runMCP runs `vlx mcp` with requests on stdin until the responses with
// the given count of ids have been written.
func runMCP(t *testing.T, requests []string, responses int, args ...string) result {
	t.Helper()
	var out, errOut syncBuffer
	v := viper.New()
	config.SetDefaults(v)
	stdin := &heldReader{r: strings.NewReader(strings.Join(requests, "\n") + "\n"),
		done: func() bool { return strings.Count(out.String(), `"id":`) >= responses }}
	a := &app{v: v, in: stdin, out: &out, errOut: &errOut, dirs: []string{}}
	a.tty = func(any) bool { return false }
	cmd := newRootCommand(a)
	cmd.SetArgs(append([]string{"mcp"}, args...))
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		errOut.WriteString("vlx: " + err.Error() + "\n")
	}
	return result{out.String(), errOut.String(), ExitCode(err)}
}

// mcpInit is the handshake every session starts with.
var mcpInit = []string{
	`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"1"}}}`,
	`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
}

// TestMCPCommand serves a session over stdio against the demo.
func TestMCPCommand(t *testing.T) {
	sock := startDemo(t)
	reqs := append(append([]string(nil), mcpInit...),
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"call","arguments":{"service":"io.vlx.demo","method":"GetGreeting","params":{"name":"Ada"}}}}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"call","arguments":{"service":"io.vlx.demo","method":"SetDefaultLanguage","params":{"language":"german"}}}}`,
	)
	r := runMCP(t, reqs, 3, "--search", filepath.Dir(sock), "--allow-writes")
	if r.code != 0 {
		t.Fatalf("exit %d: %s", r.code, r.err)
	}
	for _, want := range []string{`"serverInfo":{"name":"vlx"`, `Ada`, `\"kind\":\"refused\"`} {
		if !strings.Contains(r.out, want) {
			t.Errorf("stdout lacks %s:\n%s", want, r.out)
		}
	}
	if !strings.Contains(r.err, "--allow-writes has no effect yet") {
		t.Errorf("stderr = %q", r.err)
	}
}

// TestMCPCommandArgs rejects arguments.
func TestMCPCommandArgs(t *testing.T) {
	if r := run(t, "", "mcp", "extra"); r.code != ExitUsage {
		t.Errorf("exit %d, want usage: %s", r.code, r.err)
	}
}

// TestMCPCommandBadInput reports a broken stream.
func TestMCPCommandBadInput(t *testing.T) {
	r := runMCP(t, []string{"not json"}, 0)
	if r.code == 0 || !strings.Contains(r.err, "mcp server:") || r.out != "" {
		t.Errorf("result = %+v", r)
	}
}

// TestIsDisconnect recognises a client going away.
func TestIsDisconnect(t *testing.T) {
	if !isDisconnect(io.EOF) || isDisconnect(context.Canceled) {
		t.Error("isDisconnect")
	}
	if (nopWriteCloser{io.Discard}).Close() != nil {
		t.Error("Close")
	}
}
