package discovery

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultDirs(t *testing.T) {
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")
	dirs := DefaultDirs()
	joined := strings.Join(dirs, " ")
	for _, want := range []string{"/run/systemd", "/run/varlink/registry", "/run/user/1000/systemd"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %s in %v", want, dirs)
		}
	}
	t.Setenv("XDG_RUNTIME_DIR", "relative")
	for _, d := range DefaultDirs() {
		if strings.HasPrefix(d, "relative") {
			t.Error("relative XDG_RUNTIME_DIR used")
		}
	}
	// Discover with the defaults must not fail, whatever the host has.
	_ = Discover(Options{})
}

func TestAmbiguousErrorMessage(t *testing.T) {
	err := &AmbiguousError{Query: "m", Candidates: []Socket{{Path: "/a"}, {Path: "/b"}}}
	if msg := err.Error(); !strings.Contains(msg, "matches 2") || !strings.Contains(msg, "/a") {
		t.Errorf("message %q", msg)
	}
}

func TestResolveRejectsNonSockets(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Resolve(f, nil); err == nil || !strings.Contains(err.Error(), "not a Unix socket") {
		t.Errorf("err = %v", err)
	}
	if _, err := Resolve("unix:"+f, nil); err == nil {
		t.Error("unix: file accepted")
	}
}

func TestDiscoverSkipsSymlinkedDirs(t *testing.T) {
	real := shortTempDir(t)
	listen(t, filepath.Join(real, "io.x.Y"))
	link := filepath.Join(shortTempDir(t), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got := Discover(Options{Dirs: []string{link}}); len(got) != 0 {
		t.Errorf("followed a symlinked directory: %v", got)
	}
}
