package discovery

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
)

func listen(t *testing.T, path string) {
	t.Helper()
	l, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
}

func shortTempDir(t *testing.T) string {
	t.Helper()
	// Unix socket paths are limited to ~104 bytes; t.TempDir can be long.
	dir, err := os.MkdirTemp("", "vlx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func TestDiscover(t *testing.T) {
	a := shortTempDir(t)
	b := shortTempDir(t)
	listen(t, filepath.Join(a, "io.systemd.Hostname"))
	listen(t, filepath.Join(a, "io.systemd.Resolve"))
	if err := os.WriteFile(filepath.Join(a, "not-a-socket"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(a, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	listen(t, filepath.Join(a, "sub", "io.too.Deep"))
	// A registry-style symlink to a socket already found is de-duplicated.
	if err := os.Symlink(filepath.Join(a, "io.systemd.Hostname"), filepath.Join(b, "io.systemd.Hostname")); err != nil {
		t.Fatal(err)
	}
	listen(t, filepath.Join(b, "com.example.Zed"))

	got := Discover(Options{Dirs: []string{a, "/nonexistent"}, Extra: []string{b}})
	var names []string
	for _, s := range got {
		names = append(names, s.Name)
		if !s.Accessible {
			t.Errorf("%s not accessible", s.Path)
		}
	}
	want := []string{"com.example.Zed", "io.systemd.Hostname", "io.systemd.Resolve"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("got %v, want %v", names, want)
		}
	}
}

func TestResolve(t *testing.T) {
	dir := shortTempDir(t)
	host := filepath.Join(dir, "io.systemd.Hostname")
	listen(t, host)
	socks := []Socket{
		{Path: host, Name: "io.systemd.Hostname"},
		{Path: "/run/systemd/resolve/io.systemd.Resolve", Name: "io.systemd.Resolve"},
		{Path: "/run/systemd/resolve/io.systemd.Resolve.Monitor", Name: "io.systemd.Resolve.Monitor"},
		{Path: "/run/systemd/io.systemd.Machine", Name: "io.systemd.Machine"},
		{Path: "/run/systemd/io.systemd.MachineImage", Name: "io.systemd.MachineImage"},
	}
	tests := []struct {
		arg     string
		want    string
		wantErr error
		ambig   bool
	}{
		{arg: host, want: host},
		{arg: "unix:" + host, want: host},
		{arg: "unix:" + host + ";mode=0666", want: host},
		{arg: "io.systemd.Hostname", want: host},
		{arg: "hostname", want: host},
		{arg: "HOST", want: host},
		{arg: "resolve", want: "/run/systemd/resolve/io.systemd.Resolve"}, // short name wins over fragment
		{arg: "monitor", want: "/run/systemd/resolve/io.systemd.Resolve.Monitor"},
		{arg: "machine", want: "/run/systemd/io.systemd.Machine"},
		{arg: "mach", ambig: true},
		{arg: "nope", wantErr: ErrNotFound},
		{arg: dir + "/missing", wantErr: ErrNotFound},
		{arg: "", wantErr: ErrNotFound},
	}
	for _, tt := range tests {
		got, err := Resolve(tt.arg, socks)
		switch {
		case tt.ambig:
			var ae *AmbiguousError
			if !errors.As(err, &ae) {
				t.Errorf("Resolve(%q) err = %v, want ambiguous", tt.arg, err)
			}
		case tt.wantErr != nil:
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Resolve(%q) err = %v, want %v", tt.arg, err, tt.wantErr)
			}
		case err != nil:
			t.Errorf("Resolve(%q): %v", tt.arg, err)
		case got != tt.want:
			t.Errorf("Resolve(%q) = %q, want %q", tt.arg, got, tt.want)
		}
	}
}

func TestShortName(t *testing.T) {
	for in, want := range map[string]string{
		"io.systemd.Hostname":        "Hostname",
		"io.systemd.Resolve.Monitor": "Resolve.Monitor",
		"io.systemd.":                "io.systemd.",
		"com.example.Foo":            "com.example.Foo",
	} {
		if got := ShortName(in); got != want {
			t.Errorf("ShortName(%q) = %q, want %q", in, got, want)
		}
	}
}
