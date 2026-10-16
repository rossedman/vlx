package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultPath(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "/x/cfg")
	if got := DefaultPath(); got != "/x/cfg/vlx/config.yaml" {
		t.Errorf("got %s", got)
	}
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", "relative")
	t.Setenv("HOME", home)
	if got := DefaultPath(); got != filepath.Join(home, ".config", "vlx", "config.yaml") {
		t.Errorf("got %s", got)
	}
}

func TestDefaultFileIsLoaded(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	p := filepath.Join(dir, "vlx", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("max_replies: 7\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(newViper(), "")
	if err != nil || c.MaxReplies != 7 || c.File != p {
		t.Fatalf("%+v %v", c, err)
	}
}

func TestBadFiles(t *testing.T) {
	dir := t.TempDir()
	if err := CheckPermissions(dir); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Errorf("directory: %v", err)
	}
	p := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(p, []byte("timeout: [\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(newViper(), p); err == nil {
		t.Error("malformed YAML loaded")
	}
	if err := os.WriteFile(p, []byte("timeout: soon\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(newViper(), p); err == nil {
		t.Error("bad duration loaded")
	}
	t.Setenv("NO_COLOR", "1")
	if err := os.WriteFile(p, []byte("output: json\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(newViper(), p)
	if err != nil || !c.NoColor {
		t.Errorf("NO_COLOR: %+v %v", c, err)
	}
	if err := os.WriteFile(p, []byte("output: xml\nmax_frame_size: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(newViper(), p); err == nil || !strings.Contains(err.Error(), "output") || !strings.Contains(err.Error(), "max_frame_size") {
		t.Errorf("validation: %v", err)
	}
}
