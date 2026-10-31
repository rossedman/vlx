package mcpserver

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/demo"
	"github.com/rossedman/vlx/internal/discovery"
)

// session is a connected MCP client talking to a Server in memory.
type session struct {
	t  *testing.T
	cs *mcp.ClientSession
}

// startDemo runs the demo service for the test and returns its socket path.
func startDemo(t *testing.T) string {
	t.Helper()
	srv, err := demo.ListenTemp()
	if err != nil {
		t.Fatalf("demo: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv.Path()
}

// connect starts a Server with opts, searching only dirs, and connects a
// client to it.
func connect(t *testing.T, opts Options, dirs ...string) *session {
	t.Helper()
	if dirs == nil {
		dirs = []string{}
	}
	opts.Discovery = discovery.Options{Dirs: dirs}
	if opts.Client.Timeout == 0 {
		opts.Client = client.Options{Timeout: 5 * time.Second}
	}
	srv := New(opts)
	ct, st := mcp.NewInMemoryTransports()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Run(ctx, st) }()
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil).Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() {
		_ = cs.Close()
		cancel()
		<-done
	})
	return &session{t: t, cs: cs}
}

// demoSession connects to a server that finds only the demo service.
func demoSession(t *testing.T, opts Options) *session {
	t.Helper()
	return connect(t, opts, filepath.Dir(startDemo(t)))
}

// call runs a tool and returns its result decoded from the text content,
// and whether it is an error.
func (s *session) call(name string, args any) (map[string]any, bool) {
	s.t.Helper()
	res, err := s.cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		s.t.Fatalf("%s: %v", name, err)
	}
	if len(res.Content) != 1 {
		s.t.Fatalf("%s: %d content items", name, len(res.Content))
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		s.t.Fatalf("%s: content is %T", name, res.Content[0])
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		s.t.Fatalf("%s: result is not JSON: %v\n%s", name, err, text.Text)
	}
	return out, res.IsError
}

// ok runs a tool that must succeed.
func (s *session) ok(name string, args any) map[string]any {
	s.t.Helper()
	out, isErr := s.call(name, args)
	if isErr {
		s.t.Fatalf("%s failed: %v", name, out)
	}
	return out
}

// fail runs a tool that must fail with kind and returns the error object.
func (s *session) fail(name string, args any, kind string) map[string]any {
	s.t.Helper()
	out, isErr := s.call(name, args)
	if !isErr {
		s.t.Fatalf("%s succeeded, want %s: %v", name, kind, out)
	}
	e, _ := out["error"].(map[string]any)
	if e["kind"] != kind {
		s.t.Fatalf("%s: kind %v, want %s: %v", name, e["kind"], kind, e)
	}
	return e
}

// TestToolsList checks the tools and their annotations.
func TestToolsList(t *testing.T) {
	s := connect(t, Options{})
	res, err := s.cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]*mcp.Tool{}
	for _, tool := range res.Tools {
		got[tool.Name] = tool
	}
	for _, name := range []string{"list_services", "describe", "method_schema", "call"} {
		tool := got[name]
		if tool == nil {
			t.Fatalf("missing tool %s", name)
		}
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != (name != "call") {
			t.Errorf("%s annotations = %+v", name, tool.Annotations)
		}
	}
	if len(got) != 4 {
		t.Errorf("tools = %v", got)
	}
	schema, _ := json.Marshal(got["call"].InputSchema)
	if !strings.Contains(string(schema), `"params":{"description":"the method's parameters; see method_schema","type":"object"}`) {
		t.Errorf("call schema = %s", schema)
	}
}

// TestListServices lists the demo, with and without probing.
func TestListServices(t *testing.T) {
	s := demoSession(t, Options{})
	out := s.ok("list_services", map[string]any{})
	svcs := out["services"].([]any)
	if len(svcs) != 1 || dig(t, svcs[0], "name") != "io.vlx.demo" || dig(t, svcs[0], "accessible") != true {
		t.Fatalf("services = %v", svcs)
	}
	if out["notice"] != Notice || dig(t, svcs[0], "vendor") != nil {
		t.Errorf("out = %v", out)
	}
	probed := s.ok("list_services", map[string]any{"probe": true})["services"].([]any)[0]
	if dig(t, probed, "vendor") != "vlx" || len(dig(t, probed, "interfaces").([]any)) != 3 {
		t.Errorf("probed = %v", probed)
	}
}

// TestDescribe describes the demo and one of its interfaces.
func TestDescribe(t *testing.T) {
	s := demoSession(t, Options{})
	out := s.ok("describe", map[string]any{"service": "demo"})
	if n := len(out["interfaces"].([]any)); n != 3 {
		t.Fatalf("%d interfaces", n)
	}
	one := s.ok("describe", map[string]any{"service": "io.vlx.demo", "interface": "greeter"})
	ifaces := one["interfaces"].([]any)
	if len(ifaces) != 1 || dig(t, ifaces[0], "name") != "io.vlx.demo.Greeter" {
		t.Fatalf("interfaces = %v", ifaces)
	}
	methods := map[string]map[string]any{}
	for _, m := range dig(t, ifaces[0], "methods").([]any) {
		methods[dig(t, m, "name").(string)] = m.(map[string]any)
	}
	if m := methods["GetGreeting"]; m["class"] != "READ" || m["callable"] != true || m["streaming"] != "single" ||
		m["signature"] != "GetGreeting(name: string, language: ?Language) -> (greeting: string, language: Language)" {
		t.Errorf("GetGreeting = %v", m)
	}
	if m := methods["WatchCountdown"]; m["streaming"] != "requires-more" {
		t.Errorf("WatchCountdown = %v", m)
	}
	if m := methods["SetDefaultLanguage"]; m["class"] != "WRITE" || m["callable"] != false ||
		!strings.Contains(m["refusal"].(string), "read-only") {
		t.Errorf("SetDefaultLanguage = %v", m)
	}
	if len(dig(t, ifaces[0], "errors").([]any)) != 1 || len(dig(t, ifaces[0], "types").([]any)) != 1 {
		t.Errorf("greeter = %v", ifaces[0])
	}
	s.fail("describe", map[string]any{"service": "demo", "interface": "Nope"}, KindNotFound)
	s.fail("describe", map[string]any{"service": "nothing-here"}, KindNotFound)
	s.fail("describe", map[string]any{"service": "/no/such/socket"}, KindNotFound)
}

// TestMethodSchema checks a schema, its example and optional fields.
func TestMethodSchema(t *testing.T) {
	s := demoSession(t, Options{})
	out := s.ok("method_schema", map[string]any{"service": "demo", "method": "GetGreeting"})
	if out["method"] != "io.vlx.demo.Greeter.GetGreeting" || out["callable"] != true {
		t.Fatalf("out = %v", out)
	}
	if got := dig(t, out, "inputSchema", "required").([]any); len(got) != 1 || got[0] != "name" {
		t.Errorf("required = %v", got)
	}
	if got := dig(t, out, "example", "name"); got != "" {
		t.Errorf("example = %v", out["example"])
	}
	if got := out["optional"].([]any); len(got) != 1 || !strings.HasPrefix(got[0].(string), "language") {
		t.Errorf("optional = %v", got)
	}
	if got := dig(t, out, "outputSchema", "properties", "language", "$ref"); got != "#/$defs/Language" {
		t.Errorf("output language = %v", got)
	}
	s.fail("method_schema", map[string]any{"service": "demo", "method": "Nope"}, KindNotFound)
	s.fail("method_schema", map[string]any{"service": "nope", "method": "Nope"}, KindNotFound)
}

// TestCallRead sends READ calls and checks replies and errors.
func TestCallRead(t *testing.T) {
	s := demoSession(t, Options{})
	out := s.ok("call", map[string]any{"service": "demo", "method": "GetGreeting", "params": map[string]any{"name": "Ada"}})
	replies := out["replies"].([]any)
	if len(replies) != 1 || !strings.Contains(dig(t, replies[0], "greeting").(string), "Ada") {
		t.Fatalf("replies = %v", replies)
	}
	if out["class"] != "READ" || out["continues"] != false || out["notice"] != Notice {
		t.Errorf("out = %v", out)
	}

	e := s.fail("call", map[string]any{"service": "demo", "method": "GetGreeting", "params": map[string]any{"name": ""}}, KindVarlink)
	if dig(t, e, "varlink", "name") != "io.vlx.demo.Greeter.EmptyName" || dig(t, e, "varlink", "doc") != "The name was empty." {
		t.Errorf("error = %v", e)
	}

	e = s.fail("call", map[string]any{"service": "demo", "method": "GetGreeting", "params": map[string]any{"nam": 1, "language": "klingon"}}, KindInvalidParams)
	if n := len(e["params"].([]any)); n < 3 {
		t.Errorf("want every problem listed, got %v", e["params"])
	}
	s.fail("call", map[string]any{"service": "demo", "method": "GetGreeting", "params": map[string]any{"name": "a"}, "stream": map[string]any{}}, KindInvalidParams)
	s.fail("call", map[string]any{"service": "demo", "method": "Missing"}, KindNotFound)

	out = s.ok("call", map[string]any{"service": "demo", "method": "ListLanguages"})
	if dig(t, out["replies"].([]any)[0], "default") == nil {
		t.Errorf("ListLanguages = %v", out)
	}
}

// TestCallStreams checks the stream limits on the demo's streaming methods.
func TestCallStreams(t *testing.T) {
	s := demoSession(t, Options{})
	countdown := func(from int, stream map[string]any) map[string]any {
		args := map[string]any{"service": "demo", "method": "WatchCountdown",
			"params": map[string]any{"from": from, "intervalMilliseconds": 50}}
		if stream != nil {
			args["stream"] = stream
		}
		return s.ok("call", args)
	}

	out := countdown(50, map[string]any{"max_replies": 3})
	if len(out["replies"].([]any)) != 3 || out["continues"] != true || out["stopped"] != StoppedMaxReplies {
		t.Errorf("capped = %v", out)
	}
	out = countdown(2, nil) // requires more, so it streams without asking
	if len(out["replies"].([]any)) != 3 || out["continues"] != false || out["stopped"] != nil {
		t.Errorf("full = %v", out)
	}
	out = s.ok("call", map[string]any{"service": "demo", "method": "WatchCountdown",
		"params": map[string]any{"from": 100, "intervalMilliseconds": 5000}, "stream": map[string]any{"max_seconds": 1}})
	if len(out["replies"].([]any)) != 1 || out["continues"] != true || out["stopped"] != StoppedMaxSeconds {
		t.Errorf("timed = %v", out)
	}

	one := s.ok("call", map[string]any{"service": "demo", "method": "ListItems"})
	all := s.ok("call", map[string]any{"service": "demo", "method": "ListItems", "stream": map[string]any{"max_replies": 200}})
	if len(one["replies"].([]any)) != 1 || len(all["replies"].([]any)) < 2 || all["continues"] != false {
		t.Errorf("one = %v\nall = %v", one, all)
	}
}

// TestCallRedacts checks that secrets in replies are hidden.
func TestCallRedacts(t *testing.T) {
	s := demoSession(t, Options{})
	out := s.ok("call", map[string]any{"service": "demo", "method": "GetSupplier", "params": map[string]any{"id": 1}})
	if got := dig(t, out["replies"].([]any)[0], "supplier", "apiToken"); got != "«redacted»" {
		t.Errorf("apiToken = %v", got)
	}
	s = demoSession(t, Options{Redact: []string{}})
	out = s.ok("call", map[string]any{"service": "demo", "method": "GetSupplier", "params": map[string]any{"id": 1}})
	if got := dig(t, out["replies"].([]any)[0], "supplier", "apiToken"); got == "«redacted»" {
		t.Errorf("redaction off but apiToken = %v", got)
	}
}

// TestCallRefuses checks that nothing but READ is sent, whatever the flags.
func TestCallRefuses(t *testing.T) {
	for _, allow := range []bool{false, true} {
		s := demoSession(t, Options{AllowWrites: allow})
		e := s.fail("call", map[string]any{"service": "demo", "method": "SetDefaultLanguage", "params": map[string]any{"language": "german"}}, KindRefused)
		if e["class"] != "WRITE" || e["retry"] != false || !strings.Contains(e["message"].(string), "Nothing was sent") {
			t.Errorf("allow=%v: %v", allow, e)
		}
		e = s.fail("call", map[string]any{"service": "demo", "method": "RemoveItem", "params": map[string]any{"id": 1}}, KindRefused)
		if e["class"] != "DANGER" {
			t.Errorf("allow=%v: %v", allow, e)
		}
		// The language did not change.
		out := s.ok("call", map[string]any{"service": "demo", "method": "ListLanguages"})
		if got := dig(t, out["replies"].([]any)[0], "default"); got != "english" {
			t.Errorf("default language = %v", got)
		}
	}
}

// TestResources lists and reads the interface resources.
func TestResources(t *testing.T) {
	s := demoSession(t, Options{})
	ctx := context.Background()
	res, err := s.cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) != 3 || res.Resources[0].URI != "varlink://io.vlx.demo/io.vlx.demo.Greeter" || res.Resources[0].MIMEType != "text/plain" {
		t.Fatalf("resources = %+v", res.Resources)
	}
	read, err := s.cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: res.Resources[0].URI})
	if err != nil {
		t.Fatal(err)
	}
	if text := read.Contents[0].Text; !strings.HasPrefix(text, "# "+Notice) || !strings.Contains(text, "interface io.vlx.demo.Greeter") {
		t.Errorf("text = %q", text)
	}
	for _, uri := range []string{"varlink://io.vlx.demo/io.vlx.demo.Nope", "varlink://nope/io.vlx.demo.Greeter", "varlink://io.vlx.demo/a/b"} {
		if _, err := s.cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri}); err == nil {
			t.Errorf("%s: read succeeded", uri)
		}
	}
	tmpl, err := s.cs.ListResourceTemplates(ctx, nil)
	if err != nil || len(tmpl.ResourceTemplates) != 1 {
		t.Errorf("templates = %v, %v", tmpl, err)
	}
	if _, err := s.cs.ListTools(ctx, nil); err != nil {
		t.Errorf("middleware broke other methods: %v", err)
	}
}

// TestLimitsFromConfig checks that configured defaults apply.
func TestLimitsFromConfig(t *testing.T) {
	s := demoSession(t, Options{MCP: config.MCP{MaxStreamReplies: 2}})
	out := s.ok("call", map[string]any{"service": "demo", "method": "WatchCountdown",
		"params": map[string]any{"from": 10, "intervalMilliseconds": 50}})
	if len(out["replies"].([]any)) != 2 || out["stopped"] != StoppedMaxReplies {
		t.Errorf("out = %v", out)
	}
}
