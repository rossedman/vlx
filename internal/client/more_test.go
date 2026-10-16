package client_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/rossedman/sdvarlink/varlink"

	"github.com/rossedman/vlx/internal/client"
)

func TestHints(t *testing.T) {
	for _, err := range []error{
		syscall.EACCES, syscall.ECONNREFUSED, syscall.ENOENT, context.DeadlineExceeded,
		varlink.ErrMethodNotFound, varlink.ErrInvalidParameter, varlink.ErrExpectedMore,
		varlink.ErrFrameTooLarge, varlink.ErrUnsupported, os.ErrPermission,
	} {
		if client.Hint(err) == "" {
			t.Errorf("no hint for %v", err)
		}
	}
	if client.Hint(nil) != "" || client.Hint(errors.New("x")) != "" {
		t.Error("unexpected hint")
	}
}

func TestAsVarlinkError(t *testing.T) {
	d, ok := client.AsVarlinkError(&varlink.SystemError{Errno: 2, ErrnoName: "ENOENT"})
	if !ok || d.Name != "io.systemd.System" || !strings.Contains(string(d.Parameters), "ENOENT") {
		t.Fatalf("%+v %v", d, ok)
	}
	if _, ok := client.AsVarlinkError(errors.New("x")); ok {
		t.Error("plain error matched")
	}
}

func TestInfoAndDefaults(t *testing.T) {
	path := startDemo(t)
	info, err := client.Info(context.Background(), path, client.Options{})
	if err != nil || info.Vendor != "vlx" {
		t.Fatalf("%+v %v", info, err)
	}
	if _, err := client.Info(context.Background(), "/nonexistent", client.Options{}); err == nil {
		t.Error("Info on a missing socket succeeded")
	}
	if _, err := client.Introspect(context.Background(), "/nonexistent", opts); err == nil {
		t.Error("Introspect on a missing socket succeeded")
	}
	err = client.Call(context.Background(), "/nonexistent", client.Request{Method: "a.B.C"}, opts, nil)
	if err == nil {
		t.Error("Call on a missing socket succeeded")
	}
}

func TestCompact(t *testing.T) {
	if got, _ := client.Compact([]byte("  ")); string(got) != "{}" {
		t.Errorf("empty = %s", got)
	}
	if got, _ := client.Compact([]byte("{ \"a\" : 1 }")); string(got) != `{"a":1}` {
		t.Errorf("got %s", got)
	}
}

func TestCallbackErrors(t *testing.T) {
	path := startDemo(t)
	boom := errors.New("boom")
	err := client.Call(context.Background(), path, client.Request{Method: "io.vlx.demo.Greeter.ListLanguages"}, opts,
		func(client.Reply) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("single: %v", err)
	}
	err = client.Call(context.Background(), path, client.Request{Method: "io.vlx.demo.Greeter.ListLanguages"}, opts,
		func(client.Reply) error { return client.ErrStop })
	if err != nil {
		t.Errorf("single ErrStop: %v", err)
	}
	err = client.Call(context.Background(), path, client.Request{Method: "io.vlx.demo.Inventory.ListItems", More: true}, opts,
		func(client.Reply) error { return boom })
	if !errors.Is(err, boom) {
		t.Errorf("stream: %v", err)
	}
}
