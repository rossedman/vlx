package demo

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// rawClient speaks the wire protocol directly, to reach paths a
// well-behaved client never takes.
type rawClient struct {
	t *testing.T
	c net.Conn
	r *bufio.Reader
}

func dialRaw(t *testing.T, path string) *rawClient {
	t.Helper()
	c, err := net.Dial("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	return &rawClient{t: t, c: c, r: bufio.NewReader(c)}
}

func (rc *rawClient) send(msg string) {
	rc.t.Helper()
	if _, err := rc.c.Write(append([]byte(msg), 0)); err != nil {
		rc.t.Fatal(err)
	}
}

func (rc *rawClient) recv() map[string]any {
	rc.t.Helper()
	_ = rc.c.SetReadDeadline(time.Now().Add(3 * time.Second))
	b, err := rc.r.ReadBytes(0)
	if err != nil {
		rc.t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(bytes.TrimSuffix(b, []byte{0}), &m); err != nil {
		rc.t.Fatal(err)
	}
	return m
}

func (rc *rawClient) call(msg string) map[string]any {
	rc.t.Helper()
	rc.send(msg)
	return rc.recv()
}

func start(t *testing.T) *Server {
	t.Helper()
	s, err := ListenTemp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func errName(m map[string]any) string {
	s, _ := m["error"].(string)
	return s
}

func TestProtocolErrors(t *testing.T) {
	rc := dialRaw(t, start(t).Path())
	for msg, want := range map[string]string{
		`{"method":"NoDot"}`:                                                                                           "org.varlink.service.InterfaceNotFound",
		`{"method":"com.example.Nope.X"}`:                                                                              "org.varlink.service.InterfaceNotFound",
		`{"method":"io.vlx.demo.Greeter.Nope"}`:                                                                        "org.varlink.service.MethodNotFound",
		`{"method":"io.vlx.demo.Greeter.GetGreeting","parameters":{}}`:                                                 "org.varlink.service.InvalidParameter",
		`{"method":"io.vlx.demo.Greeter.WatchCountdown","parameters":{"from":1}}`:                                      "org.varlink.service.ExpectedMore",
		`{"method":"io.vlx.demo.Greeter.WatchCountdown","parameters":{"from":101},"more":true}`:                        "org.varlink.service.InvalidParameter",
		`{"method":"io.vlx.demo.Greeter.WatchCountdown","parameters":{"from":1,"intervalMilliseconds":1},"more":true}`: "org.varlink.service.InvalidParameter",
		`{"method":"org.varlink.service.GetInterfaceDescription","parameters":{"interface":"x.Y"}}`:                    "org.varlink.service.InterfaceNotFound",
		`{"method":"io.vlx.demo.Inventory.ListItems","parameters":{"tag":"none"}}`:                                     "io.vlx.demo.Inventory.NoSuchItem",
		`{"method":"io.vlx.demo.Inventory.RemoveItem","parameters":{"id":99}}`:                                         "io.vlx.demo.Inventory.NoSuchItem",
	} {
		if got := errName(rc.call(msg)); got != want {
			t.Errorf("%s: error %q, want %q", msg, got, want)
		}
	}
}

func TestInventoryLifecycle(t *testing.T) {
	rc := dialRaw(t, start(t).Path())
	r := rc.call(`{"method":"io.vlx.demo.Inventory.AddItem","parameters":{"name":"Bolt","quantity":9}}`)
	item := r["parameters"].(map[string]any)["item"].(map[string]any)
	if item["id"] != float64(5) || item["name"] != "Bolt" {
		t.Fatalf("AddItem = %v", r)
	}
	if errName(rc.call(`{"method":"io.vlx.demo.Inventory.RemoveItem","parameters":{"id":5}}`)) != "" {
		t.Fatal("RemoveItem failed")
	}
	if errName(rc.call(`{"method":"io.vlx.demo.Inventory.GetItem","parameters":{"id":5}}`)) == "" {
		t.Fatal("removed item still there")
	}
	rc.call(`{"method":"io.vlx.demo.Inventory.Reset"}`)
	r = rc.call(`{"method":"io.vlx.demo.Inventory.GetSupplier","parameters":{"id":2}}`)
	if !strings.Contains(r["parameters"].(map[string]any)["supplier"].(map[string]any)["email"].(string), "bell-labs") {
		t.Fatalf("GetSupplier = %v", r)
	}
	// Tag filter, streamed.
	rc.send(`{"method":"io.vlx.demo.Inventory.ListItems","parameters":{"tag":"tools"},"more":true}`)
	n := 0
	for {
		m := rc.recv()
		n++
		if c, _ := m["continues"].(bool); !c {
			break
		}
	}
	if n != 2 {
		t.Fatalf("%d tools, want 2", n)
	}
	// Oneway calls get no reply; the next call's reply comes next.
	rc.send(`{"method":"io.vlx.demo.Greeter.SetDefaultLanguage","parameters":{"language":"german"},"oneway":true}`)
	r = rc.call(`{"method":"io.vlx.demo.Greeter.GetGreeting","parameters":{"name":"X"}}`)
	if !strings.Contains(r["parameters"].(map[string]any)["greeting"].(string), "Hallo") {
		t.Fatalf("after oneway: %v", r)
	}
}

func TestBadInputClosesConnection(t *testing.T) {
	s := start(t)
	rc := dialRaw(t, s.Path())
	rc.send(`not json`)
	_ = rc.c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := rc.r.ReadByte(); err == nil {
		t.Fatal("connection stayed open after a protocol error")
	}
	rc = dialRaw(t, s.Path())
	big := bytes.Repeat([]byte("x"), maxMessage+10)
	_, _ = rc.c.Write(big)
	_ = rc.c.SetReadDeadline(time.Now().Add(3 * time.Second))
	if _, err := rc.r.ReadByte(); err == nil {
		t.Fatal("connection stayed open after an oversized message")
	}
}

func TestCloseStopsStreams(t *testing.T) {
	s := start(t)
	rc := dialRaw(t, s.Path())
	rc.send(`{"method":"io.vlx.demo.Greeter.WatchCountdown","parameters":{"from":100,"intervalMilliseconds":50},"more":true}`)
	rc.recv()
	done := make(chan error, 1)
	go func() { done <- s.Close() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close hung on a running stream")
	}
	if err := s.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if _, err := os.Stat(filepath.Dir(s.Path())); !os.IsNotExist(err) {
		t.Error("temporary directory left behind")
	}
}

func TestListenUser(t *testing.T) {
	dir, err := os.MkdirTemp("", "vlx")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	t.Setenv("XDG_RUNTIME_DIR", dir)
	want := filepath.Join(dir, "vlx", "io.vlx.demo")
	if UserSocketPath() != want {
		t.Fatalf("UserSocketPath = %s", UserSocketPath())
	}
	// A stale socket from a crashed demo is replaced.
	if err := os.MkdirAll(filepath.Dir(want), 0o700); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", want)
	if err != nil {
		t.Fatal(err)
	}
	l.(*net.UnixListener).SetUnlinkOnClose(false)
	l.Close()
	a, err := ListenUser()
	if err != nil || a.Path() != want {
		t.Fatalf("ListenUser = %v, %v", a, err)
	}
	defer a.Close()
	// A second demo falls back to a temporary socket.
	b, err := ListenUser()
	if err != nil || b.Path() == want {
		t.Fatalf("second ListenUser = %v, %v", b, err)
	}
	b.Close()
	// Without XDG_RUNTIME_DIR there is no user socket.
	t.Setenv("XDG_RUNTIME_DIR", "")
	if UserSocketPath() != "" {
		t.Error("UserSocketPath without XDG_RUNTIME_DIR")
	}
	c, err := ListenUser()
	if err != nil {
		t.Fatal(err)
	}
	c.Close()
}

func TestListenErrors(t *testing.T) {
	if _, err := Listen("/nonexistent/dir/sock"); err == nil {
		t.Error("Listen in a missing directory succeeded")
	}
	removeStaleSocket("/nonexistent") // no-op
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	removeStaleSocket(f)
	if _, err := os.Stat(f); err != nil {
		t.Error("removeStaleSocket removed a regular file")
	}
}
