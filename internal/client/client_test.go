package client_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rossedman/sdvarlink/varlink"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/demo"
)

func startDemo(t *testing.T) string {
	t.Helper()
	srv, err := demo.ListenTemp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { srv.Close() })
	return srv.Path()
}

var opts = client.Options{Timeout: 5 * time.Second}

func TestIntrospect(t *testing.T) {
	path := startDemo(t)
	svc, err := client.Introspect(context.Background(), path, opts)
	if err != nil {
		t.Fatal(err)
	}
	if svc.Info.Product != "vlx demo service" {
		t.Errorf("product = %q", svc.Info.Product)
	}
	var names []string
	for _, i := range svc.Interfaces {
		if i.Err != nil {
			t.Errorf("%s: %v", i.Name, i.Err)
		}
		names = append(names, i.Name)
	}
	if got := strings.Join(names, ","); got != "io.vlx.demo.Greeter,io.vlx.demo.Inventory,org.varlink.service" {
		t.Errorf("interfaces = %s", got)
	}

	iface, m, err := svc.FindMethod("GetGreeting")
	if err != nil || iface.Name != "io.vlx.demo.Greeter" || m.Name != "GetGreeting" {
		t.Errorf("FindMethod bare: %v %v %v", iface, m, err)
	}
	if _, _, err := svc.FindMethod("io.vlx.demo.Inventory.GetItem"); err != nil {
		t.Error(err)
	}
	for _, bad := range []string{"Nope", "io.vlx.demo.Nope.X", "io.vlx.demo.Greeter.Nope"} {
		if _, _, err := svc.FindMethod(bad); err == nil {
			t.Errorf("FindMethod(%q) succeeded", bad)
		}
	}
}

func collect(t *testing.T, path string, req client.Request) ([]client.Reply, error) {
	t.Helper()
	var out []client.Reply
	err := client.Call(context.Background(), path, req, opts, func(r client.Reply) error {
		out = append(out, r)
		return nil
	})
	return out, err
}

func TestCall(t *testing.T) {
	path := startDemo(t)
	replies, err := collect(t, path, client.Request{
		Method: "io.vlx.demo.Greeter.GetGreeting",
		Params: json.RawMessage(`{"name": "Ada", "language": "german"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 1 || !strings.Contains(string(replies[0].Params), "Hallo, Ada!") {
		t.Fatalf("replies = %+v", replies)
	}
}

func TestCallErrors(t *testing.T) {
	path := startDemo(t)
	_, err := collect(t, path, client.Request{Method: "io.vlx.demo.Greeter.GetGreeting", Params: json.RawMessage(`{"name": ""}`)})
	d, ok := client.AsVarlinkError(err)
	if !ok || d.Name != "io.vlx.demo.Greeter.EmptyName" {
		t.Fatalf("err = %v", err)
	}
	_, err = collect(t, path, client.Request{Method: "io.vlx.demo.Inventory.GetItem", Params: json.RawMessage(`{"id": 99}`)})
	d, ok = client.AsVarlinkError(err)
	if !ok || d.Name != "io.vlx.demo.Inventory.NoSuchItem" || string(d.Parameters) != `{"id":99}` {
		t.Fatalf("err = %v, detail %+v", err, d)
	}
	_, err = collect(t, path, client.Request{Method: "io.vlx.demo.Greeter.WatchCountdown", Params: json.RawMessage(`{"from": 1}`)})
	if !errors.Is(err, varlink.ErrExpectedMore) {
		t.Fatalf("err = %v, want ExpectedMore", err)
	}
	_, err = collect(t, path, client.Request{Method: "io.vlx.demo.Greeter.GetGreeting", Params: json.RawMessage(`{"nom": "x"}`)})
	if !errors.Is(err, varlink.ErrInvalidParameter) {
		t.Fatalf("err = %v, want InvalidParameter", err)
	}
	_, err = collect(t, path, client.Request{Method: "io.vlx.demo.Greeter.GetGreeting", Params: json.RawMessage(`{bad`)})
	if err == nil || !strings.Contains(err.Error(), "parameters") {
		t.Fatalf("err = %v", err)
	}
}

func TestStream(t *testing.T) {
	path := startDemo(t)
	replies, err := collect(t, path, client.Request{
		Method: "io.vlx.demo.Greeter.WatchCountdown",
		Params: json.RawMessage(`{"from": 3, "intervalMilliseconds": 50}`),
		More:   true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(replies) != 4 || replies[3].Continues || !replies[0].Continues {
		t.Fatalf("replies = %+v", replies)
	}

	// Stopping early.
	n := 0
	err = client.Call(context.Background(), path, client.Request{
		Method: "io.vlx.demo.Inventory.ListItems", More: true,
	}, opts, func(client.Reply) error {
		n++
		return client.ErrStop
	})
	if err != nil || n != 1 {
		t.Fatalf("n = %d, err = %v", n, err)
	}
}

func TestStreamCancel(t *testing.T) {
	path := startDemo(t)
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	err := client.Call(ctx, path, client.Request{
		Method: "io.vlx.demo.Greeter.WatchCountdown",
		Params: json.RawMessage(`{"from": 100, "intervalMilliseconds": 50}`),
		More:   true,
	}, opts, func(client.Reply) error {
		n++
		if n == 2 {
			cancel()
		}
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestStateAndOneway(t *testing.T) {
	path := startDemo(t)
	if err := client.Call(context.Background(), path, client.Request{
		Method: "io.vlx.demo.Greeter.SetDefaultLanguage",
		Params: json.RawMessage(`{"language": "spanish"}`),
		Oneway: true,
	}, opts, nil); err != nil {
		t.Fatal(err)
	}
	// The oneway call is processed before the next call on a new connection
	// only eventually; poll briefly.
	deadline := time.Now().Add(2 * time.Second)
	for {
		replies, err := collect(t, path, client.Request{Method: "io.vlx.demo.Greeter.GetGreeting", Params: json.RawMessage(`{"name":"Bo"}`)})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(replies[0].Params), "¡Hola") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("language not changed: %s", replies[0].Params)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
