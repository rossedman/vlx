package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spf13/viper"

	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/demo"
)

type result struct {
	out, err string
	code     int
}

// run executes vlx with args against a fresh app, as main would.
func run(t *testing.T, stdin string, args ...string) result {
	t.Helper()
	return runApp(t, stdin, false, args...)
}

// runApp is run with control over whether the streams look like a
// terminal. Discovery only searches --search directories.
func runApp(t *testing.T, stdin string, tty bool, args ...string) result {
	t.Helper()
	var out, errOut syncBuffer
	v := viper.New()
	config.SetDefaults(v)
	a := &app{v: v, in: strings.NewReader(stdin), out: &out, errOut: &errOut, dirs: []string{}}
	a.tty = func(any) bool { return tty }
	cmd := newRootCommand(a)
	cmd.SetArgs(args)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	err := cmd.ExecuteContext(context.Background())
	if err != nil {
		errOut.WriteString("vlx: " + err.Error() + "\n")
	}
	return result{out.String(), errOut.String(), ExitCode(err)}
}

// syncBuffer is a bytes.Buffer safe for concurrent writes. The TUI's
// renderer and event loop both write to the output, which is safe for a
// terminal but not for a plain buffer.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) WriteString(str string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.WriteString(str)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func startDemo(t *testing.T) string {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_RUNTIME_DIR", "")
	srv, err := demo.ListenTemp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv.Path()
}

func TestCallRead(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "call", sock, "GetGreeting", `{"name":"Ada"}`)
	if r.code != 0 || strings.TrimSpace(r.out) != `{"greeting":"Hello, Ada!","language":"english"}` {
		t.Fatalf("%+v", r)
	}
}

func TestCallStdinAndFile(t *testing.T) {
	sock := startDemo(t)
	r := run(t, `{"name":"In"}`, "call", sock, "io.vlx.demo.Greeter.GetGreeting", "-")
	if r.code != 0 || !strings.Contains(r.out, "Hello, In!") {
		t.Fatalf("stdin: %+v", r)
	}
	f := filepath.Join(t.TempDir(), "p.json")
	if err := os.WriteFile(f, []byte(`{"name":"File"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r = run(t, "", "call", sock, "GetGreeting", "-f", f)
	if r.code != 0 || !strings.Contains(r.out, "Hello, File!") {
		t.Fatalf("file: %+v", r)
	}
}

func TestCallStream(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "call", sock, "WatchCountdown", `{"from":2,"intervalMilliseconds":50}`)
	lines := strings.Split(strings.TrimSpace(r.out), "\n")
	if r.code != 0 || len(lines) != 3 || !strings.Contains(r.err, "calling with --more") {
		t.Fatalf("%+v", r)
	}
	r = run(t, "", "call", sock, "ListItems", "--max-replies", "2")
	if r.code != 0 || len(strings.Split(strings.TrimSpace(r.out), "\n")) != 2 {
		t.Fatalf("max-replies: %+v", r)
	}
	r = run(t, "", "call", sock, "GetItem", `{"id":1}`, "--more")
	if r.code != ExitUsage {
		t.Fatalf("--more on non-streaming method: %+v", r)
	}
}

func TestCallExitCodes(t *testing.T) {
	sock := startDemo(t)
	tests := []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"varlink error", []string{"call", sock, "GetItem", `{"id":99}`}, ExitVarlink, "NoSuchItem"},
		{"bad params", []string{"call", sock, "GetItem", `{"id":"x"}`}, ExitUsage, ".id: expected int"},
		{"bad json", []string{"call", sock, "GetItem", `{`}, ExitUsage, "invalid JSON"},
		{"unknown method", []string{"call", sock, "Nope"}, ExitNotFound, "no interface"},
		{"unknown service", []string{"call", "no-such-service-anywhere", "X"}, ExitNotFound, "not found"},
		{"write refused", []string{"call", sock, "SetDefaultLanguage", `{"language":"german"}`}, ExitRefused, "read-only"},
		{"write needs yes", []string{"call", sock, "SetDefaultLanguage", `{"language":"german"}`, "--allow-writes"}, ExitRefused, "--yes"},
		{"danger refused", []string{"call", sock, "Reset"}, ExitRefused, "DANGER"},
		{"oneway read refused", []string{"call", sock, "ListLanguages", "--oneway"}, ExitRefused, "WRITE"},
		{"no-validate needs qualified", []string{"call", sock, "Reset", "--no-validate"}, ExitUsage, "qualified"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := run(t, "", tt.args...)
			if r.code != tt.code || !strings.Contains(r.err, tt.msg) {
				t.Fatalf("got code %d, err %q; want %d containing %q", r.code, r.err, tt.code, tt.msg)
			}
		})
	}
}

func TestCallWritesWithYes(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "call", sock, "SetDefaultLanguage", `{"language":"german"}`, "--allow-writes", "--yes")
	if r.code != 0 {
		t.Fatalf("%+v", r)
	}
	r = run(t, "", "call", sock, "GetGreeting", `{"name":"Z"}`)
	if !strings.Contains(r.out, "Hallo, Z!") {
		t.Fatalf("write had no effect: %+v", r)
	}
	t.Setenv("VLX_ALLOW_WRITES", "1")
	r = run(t, "", "call", sock, "RemoveItem", `{"id":2}`, "-y")
	if r.code != 0 {
		t.Fatalf("env allow_writes: %+v", r)
	}
}

func TestDryRunSendsNothing(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "call", sock, "Reset", "--dry-run")
	var msg map[string]any
	if r.code != 0 || json.Unmarshal([]byte(r.out), &msg) != nil || msg["method"] != "io.vlx.demo.Inventory.Reset" {
		t.Fatalf("%+v", r)
	}
	if !strings.Contains(r.err, "DANGER") {
		t.Errorf("dry run did not report the class: %s", r.err)
	}
}

func TestCallJSONError(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "call", sock, "GetItem", `{"id":99}`, "-o", "json")
	if r.code != ExitVarlink || strings.TrimSpace(r.out) != `{"error":"io.vlx.demo.Inventory.NoSuchItem","parameters":{"id":99}}` {
		t.Fatalf("%+v", r)
	}
}

func TestInfoDescribeTemplate(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "info", sock, "-o", "json")
	var info map[string]any
	if r.code != 0 || json.Unmarshal([]byte(r.out), &info) != nil || info["product"] != "vlx demo service" {
		t.Fatalf("info: %+v", r)
	}
	r = run(t, "", "describe", sock, "io.vlx.demo.Greeter")
	if r.code != 0 || !strings.HasPrefix(r.out, "# A friendly greeter") || strings.Contains(r.out, "org.varlink.service") {
		t.Fatalf("describe: %+v", r)
	}
	r = run(t, "", "describe", sock, "com.example.Missing")
	if r.code != ExitNotFound {
		t.Fatalf("describe missing: %+v", r)
	}
	r = run(t, "", "template", sock, "AddItem")
	if r.code != 0 || !strings.Contains(r.out, `"quantity": 0`) || !strings.Contains(r.err, "tags: []string") {
		t.Fatalf("template: %+v", r)
	}
}

func TestListSearch(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "list", "--search", filepath.Dir(sock), "--probe", "-o", "json")
	var entries []map[string]any
	if r.code != 0 || json.Unmarshal([]byte(r.out), &entries) != nil {
		t.Fatalf("%+v", r)
	}
	found := false
	for _, e := range entries {
		if e["path"] == sock && e["product"] == "vlx demo service" {
			found = true
		}
	}
	if !found {
		t.Fatalf("demo socket not listed: %s", r.out)
	}
	// SERVICE resolves by name through --search.
	r = run(t, "", "info", "io.vlx.demo", "--search", filepath.Dir(sock))
	if r.code != 0 {
		t.Fatalf("resolve by name: %+v", r)
	}
}

func TestExploreNeedsTerminal(t *testing.T) {
	startDemo(t)
	r := run(t, "", "explore")
	if r.code != ExitUsage || !strings.Contains(r.err, "interactive terminal") {
		t.Fatalf("%+v", r)
	}
}

func TestConfigCommands(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := filepath.Join(dir, "vlx", "config.yaml")
	if r := run(t, "", "config", "path"); strings.TrimSpace(r.out) != p {
		t.Fatalf("path: %+v", r)
	}
	if r := run(t, "", "config", "init"); r.code != 0 {
		t.Fatalf("init: %+v", r)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("config file: %v %v", fi, err)
	}
	if r := run(t, "", "config", "init"); r.code != ExitUsage {
		t.Fatalf("init twice: %+v", r)
	}
	if r := run(t, "", "config", "show", "--timeout", "3s"); r.code != 0 || !strings.Contains(r.out, "timeout: 3s") {
		t.Fatalf("show: %+v", r)
	}
	if err := os.Chmod(p, 0o666); err != nil {
		t.Fatal(err)
	}
	if r := run(t, "", "config", "show"); r.code != ExitUsage || !strings.Contains(r.err, "refusing") {
		t.Fatalf("insecure config: %+v", r)
	}
}

func TestVersion(t *testing.T) {
	r := run(t, "", "version", "-o", "json")
	var v map[string]any
	if r.code != 0 || json.Unmarshal([]byte(r.out), &v) != nil || v["goVersion"] == "" {
		t.Fatalf("%+v", r)
	}
}
