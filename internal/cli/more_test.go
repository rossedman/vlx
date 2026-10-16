package cli

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/rossedman/sdvarlink/varlink"

	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/discovery"
	"github.com/rossedman/vlx/internal/safety"
)

func TestInteractiveConfirmations(t *testing.T) {
	sock := startDemo(t)
	tests := []struct {
		name  string
		stdin string
		args  []string
		code  int
	}{
		{"write yes", "y\n", []string{"call", sock, "SetDefaultLanguage", `{"language":"german"}`, "--allow-writes"}, 0},
		{"write no", "n\n", []string{"call", sock, "SetDefaultLanguage", `{"language":"german"}`, "--allow-writes"}, ExitRefused},
		{"danger typed", "Reset\n", []string{"call", sock, "Reset", "--allow-writes"}, 0},
		{"danger wrong", "y\n", []string{"call", sock, "Reset", "--allow-writes"}, ExitRefused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := runApp(t, tt.stdin, true, tt.args...)
			if r.code != tt.code {
				t.Fatalf("code %d, want %d: %+v", r.code, tt.code, r)
			}
			if !strings.Contains(r.err, "socket  "+sock) {
				t.Errorf("prompt does not show the socket: %q", r.err)
			}
		})
	}
}

func TestPrettyOutputOnTerminal(t *testing.T) {
	sock := startDemo(t)
	r := runApp(t, "", true, "call", sock, "GetItem", `{"id":1}`, "--no-color")
	if r.code != 0 || !strings.Contains(r.out, "\n  \"item\": {") {
		t.Fatalf("%+v", r)
	}
	r = runApp(t, "", true, "info", sock)
	if r.code != 0 || !strings.Contains(r.out, "vlx demo service") {
		t.Fatalf("info: %+v", r)
	}
}

func TestExploreAndDemoRunTheTUI(t *testing.T) {
	sock := startDemo(t)
	for _, args := range [][]string{{"explore", sock}, {sock}, {"demo"}} {
		r := runApp(t, "q", true, args...)
		if r.code != 0 {
			t.Fatalf("%v: %+v", args, r)
		}
	}
	if r := runApp(t, "q", true, "explore", "no-such-service"); r.code != ExitNotFound {
		t.Fatalf("explore missing service: %+v", r)
	}
}

func TestDemoServe(t *testing.T) {
	dir, err := os.MkdirTemp("", "vlx")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	for _, extra := range [][]string{nil, {"--socket", filepath.Join(dir, "custom.sock")}} {
		ctx, cancel := context.WithCancel(context.Background())
		var out strings.Builder
		v := viper.New()
		config.SetDefaults(v)
		a := &app{v: v, in: strings.NewReader(""), out: &out, errOut: &out}
		cmd := newRootCommand(a)
		cmd.SetArgs(append([]string{"demo", "--serve"}, extra...))
		done := make(chan error, 1)
		go func() { done <- cmd.ExecuteContext(ctx) }()
		want := filepath.Join(dir, "vlx", "io.vlx.demo")
		if extra != nil {
			want = extra[1]
		}
		deadline := time.Now().Add(5 * time.Second)
		for {
			if c, err := net.Dial("unix", want); err == nil {
				c.Close()
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("demo did not listen on %s", want)
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(want); !os.IsNotExist(err) {
			t.Errorf("socket %s left behind", want)
		}
	}
}

func TestListText(t *testing.T) {
	sock := startDemo(t)
	dir := filepath.Dir(sock)
	r := runApp(t, "", true, "list", "--search", dir)
	if r.code != 0 || !strings.Contains(r.out, "NAME") || !strings.Contains(r.out, "io.vlx.demo") {
		t.Fatalf("%+v", r)
	}
	// A socket nobody listens on shows its probe error.
	dead := filepath.Join(dir, "dead.sock")
	l, err := net.Listen("unix", dead)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()
	r = run(t, "", "list", "--search", dir, "--probe")
	if r.code != 0 || !strings.Contains(r.out, "vlx demo service 1") || !strings.Contains(r.out, "✗") {
		t.Fatalf("probe: %+v", r)
	}
	r = run(t, "", "list")
	if r.code != 0 || !strings.Contains(r.err, "No Varlink sockets found") {
		t.Fatalf("empty: %+v", r)
	}
}

func TestDescribeVariants(t *testing.T) {
	sock := startDemo(t)
	r := run(t, "", "describe", sock, "-o", "json")
	if r.code != 0 || !strings.Contains(r.out, `"interface": "io.vlx.demo.Inventory"`) {
		t.Fatalf("%+v", r)
	}
	r = runApp(t, "", true, "describe", sock)
	if r.code != 0 || strings.Count(r.out, "interface ") < 3 {
		t.Fatalf("%+v", r)
	}
}

func TestCallMore(t *testing.T) {
	sock := startDemo(t)
	if r := run(t, "", "call", sock, "io.vlx.demo.Greeter.GetGreeting", `{"name":"N"}`, "--no-validate"); r.code != 0 || !strings.Contains(r.out, "Hello, N!") {
		t.Fatalf("no-validate: %+v", r)
	}
	if r := run(t, "", "call", sock, "SetDefaultLanguage", `{"language":"german"}`, "--oneway", "--allow-writes", "--yes"); r.code != 0 || !strings.Contains(r.err, "oneway") {
		t.Fatalf("oneway: %+v", r)
	}
	if r := run(t, `{"name":"S"}`, "call", sock, "GetGreeting", "-f", "-"); r.code != 0 || !strings.Contains(r.out, "S!") {
		t.Fatalf("-f -: %+v", r)
	}
	if r := run(t, "", "call", sock, "GetGreeting", `{}`, "-f", "x"); r.code != ExitUsage {
		t.Fatalf("both params: %+v", r)
	}
	if r := run(t, "", "call", sock, "GetGreeting", "-f", "/nonexistent.json"); r.code == 0 {
		t.Fatalf("missing file: %+v", r)
	}
	if r := run(t, "", "call", sock, "GetItem", `{"id":1}`, "--max-replies", "1"); r.code != 0 {
		t.Fatalf("max-replies: %+v", r)
	}
	if r := run(t, "", "template", sock, "Nope"); r.code != ExitNotFound {
		t.Fatalf("template unknown: %+v", r)
	}
	if r := run(t, "", "info", "/nonexistent/socket"); r.code != ExitNotFound {
		t.Fatalf("info missing: %+v", r)
	}
}

func TestCompletion(t *testing.T) {
	sock := startDemo(t)
	a := newApp()
	a.search = []string{filepath.Dir(sock)}
	fn := completeService(a)
	got, dir := fn(&cobra.Command{}, nil, "")
	if dir != cobra.ShellCompDirectiveNoFileComp || len(got) == 0 || !strings.HasPrefix(got[len(got)-1], "io.vlx.demo\t") && !containsPrefix(got, "io.vlx.demo\t") {
		t.Fatalf("got %v %v", got, dir)
	}
	if _, dir := fn(&cobra.Command{}, nil, "./x"); dir != cobra.ShellCompDirectiveDefault {
		t.Error("paths should complete as files")
	}
	if got, _ := fn(&cobra.Command{}, []string{"x"}, ""); got != nil {
		t.Error("only the first argument completes")
	}
	if r := run(t, "", "completion", "bash"); r.code != 0 || !strings.Contains(r.out, "bash completion") {
		t.Fatalf("completion: %d", r.code)
	}
}

func containsPrefix(list []string, p string) bool {
	for _, s := range list {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func TestGenerators(t *testing.T) {
	dir := t.TempDir()
	if r := run(t, "", "gen-docs", filepath.Join(dir, "md")); r.code != 0 {
		t.Fatalf("gen-docs: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "md", "vlx_call.md")); err != nil {
		t.Fatal(err)
	}
	if r := run(t, "", "man", filepath.Join(dir, "man")); r.code != 0 {
		t.Fatalf("man: %+v", r)
	}
	if _, err := os.Stat(filepath.Join(dir, "man", "vlx-call.1")); err != nil {
		t.Fatal(err)
	}
	// An unwritable target fails.
	f := filepath.Join(dir, "file")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if r := run(t, "", "gen-docs", filepath.Join(f, "sub")); r.code == 0 {
		t.Error("gen-docs into a file succeeded")
	}
	if r := run(t, "", "man", filepath.Join(f, "sub")); r.code == 0 {
		t.Error("man into a file succeeded")
	}
}

func TestVersionAndConfigText(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	if r := run(t, "", "version"); r.code != 0 || !strings.Contains(r.out, "vlx ") || !strings.Contains(r.out, "platform") {
		t.Fatalf("version: %+v", r)
	}
	p := filepath.Join(t.TempDir(), "c.yaml")
	if r := run(t, "", "config", "init", "--config", p); r.code != 0 {
		t.Fatalf("init: %+v", r)
	}
	if r := run(t, "", "config", "init", "--config", p, "--force"); r.code != 0 {
		t.Fatalf("init --force: %+v", r)
	}
	if r := run(t, "", "config", "path", "--config", p); strings.TrimSpace(r.out) != p {
		t.Fatalf("path: %+v", r)
	}
	if r := run(t, "", "config", "show", "--config", p, "-o", "json"); r.code != 0 || !strings.Contains(r.out, `"file": "`+p) {
		t.Fatalf("show json: %+v", r)
	}
	if r := run(t, "", "config", "show", "--config", p); r.code != 0 || !strings.Contains(r.out, "from "+p) {
		t.Fatalf("show: %+v", r)
	}
	if r := run(t, "", "config", "init", "--config", filepath.Join(p, "sub", "c.yaml")); r.code == 0 {
		t.Fatal("init under a file succeeded")
	}
	if r := run(t, "", "config"); r.code != 0 || !strings.Contains(r.out, "Available Commands") {
		t.Fatalf("config help: %+v", r)
	}
	if r := run(t, "", "--timeout", "0s", "version"); r.code != ExitUsage {
		t.Fatalf("invalid config: %+v", r)
	}
}

func TestExitCodeMapping(t *testing.T) {
	tests := []struct {
		err  error
		code int
	}{
		{nil, ExitOK},
		{&ExitError{Code: 7}, 7},
		{context.Canceled, ExitInterrupted},
		{&safety.RefusedError{Method: "a.B", Class: safety.Write}, ExitRefused},
		{discovery.ErrNotFound, ExitNotFound},
		{varlink.ErrUnsupported, ExitNotFound},
		{&varlink.Error{Name: "x.Y"}, ExitVarlink},
		{errors.New("other"), ExitFailure},
	}
	for _, tt := range tests {
		if got := ExitCode(tt.err); got != tt.code {
			t.Errorf("ExitCode(%v) = %d, want %d", tt.err, got, tt.code)
		}
	}
	if !IsSilent(&ExitError{Silent: true}) || IsSilent(errors.New("x")) {
		t.Error("IsSilent")
	}
	if (&ExitError{Code: 3}).Error() != "exit 3" {
		t.Error("ExitError without cause")
	}
	if Hint(varlink.ErrMethodNotFound) == "" {
		t.Error("no hint for MethodNotFound")
	}
	if NewRootCommand().Name() != "vlx" {
		t.Error("root name")
	}
}

func TestOutputHelpers(t *testing.T) {
	if truncate("abcdef", 3) != "abc…" || truncate("ab", 3) != "ab" {
		t.Error("truncate")
	}
	if classLabel(safety.Danger, false) != "DANGER" || classLabel(safety.Write, true) == "" || classLabel(safety.Danger, true) == "" {
		t.Error("classLabel")
	}
	if paramSuffix(nil) != "" || paramSuffix([]byte(`{"a":1}`)) != ` {"a":1}` {
		t.Error("paramSuffix")
	}
	if compactLine([]byte("{bad")) != "{bad" {
		t.Error("compactLine invalid")
	}
	if shortErr(errors.New("dial unix /x: connect: connection refused")) != "connection refused" {
		t.Error("shortErr")
	}
	var b strings.Builder
	tb := newTable(&b, "A", "B")
	tb.add("x\x1b[31m", "y")
	if err := tb.flush(true, nameStyle); err != nil || strings.Contains(b.String(), "\x1b[31m") {
		t.Errorf("table: %q %v", b.String(), err)
	}
	if styleFirstColumn("ab", 5, nameStyle) != "ab" {
		t.Error("styleFirstColumn short")
	}
}
