package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
)

func newViper() *viper.Viper {
	v := viper.New()
	SetDefaults(v)
	return v
}

func TestDefaults(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	c, err := Load(newViper(), "")
	if err != nil {
		t.Fatal(err)
	}
	if c.AllowWrites || c.Timeout != 10*time.Second || c.MaxReplies != 1000 || c.File != "" || len(c.Redact) == 0 {
		t.Errorf("unexpected defaults: %+v", c)
	}
}

func TestTemplateLoads(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte(Template), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(newViper(), p)
	if err != nil {
		t.Fatal(err)
	}
	if c.File != p || c.AllowWrites {
		t.Errorf("got %+v", c)
	}
}

func TestFileAndEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	data := "timeout: 3s\nsafety:\n  danger: [\"com.example.*\"]\n"
	if err := os.WriteFile(p, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VLX_ALLOW_WRITES", "true")
	t.Setenv("VLX_MAX_REPLIES", "5")
	c, err := Load(newViper(), p)
	if err != nil {
		t.Fatal(err)
	}
	if !c.AllowWrites || c.Timeout != 3*time.Second || c.MaxReplies != 5 || len(c.Safety.Danger) != 1 {
		t.Errorf("got %+v", c)
	}
}

func TestInsecureFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("allow_writes: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o666); err != nil {
		t.Fatal(err)
	}
	_, err := Load(newViper(), p)
	var ie *InsecureError
	if !errors.As(err, &ie) {
		t.Fatalf("err = %v, want InsecureError", err)
	}
}

func TestInvalid(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(p, []byte("timeout: -1s\nmax_replies: 0\nsafety:\n  read: [\"[\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(newViper(), p); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestMissingExplicit(t *testing.T) {
	if _, err := Load(newViper(), filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error")
	}
}

// TestMCPSettings loads and validates the mcp section.
func TestMCPSettings(t *testing.T) {
	dir := t.TempDir()
	write := func(body string) string {
		p := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	c, err := Load(newViper(), write("mcp:\n  max_stream_replies: 50\n  max_stream_seconds: 30\n  max_result_bytes: 8192\n"))
	if err != nil {
		t.Fatal(err)
	}
	if c.MCP != (MCP{MaxStreamReplies: 50, MaxStreamSeconds: 30, MaxResultBytes: 8192}) {
		t.Errorf("mcp = %+v", c.MCP)
	}
	defaults, err := Load(newViper(), "")
	if err != nil {
		t.Fatal(err)
	}
	if defaults.MCP != (MCP{MaxStreamReplies: DefaultMCPStreamReplies, MaxStreamSeconds: DefaultMCPStreamSeconds, MaxResultBytes: DefaultMCPResultBytes}) {
		t.Errorf("defaults = %+v", defaults.MCP)
	}
	_, err = Load(newViper(), write("mcp:\n  max_stream_replies: 201\n  max_stream_seconds: 0\n  max_result_bytes: 10\n"))
	if err == nil {
		t.Fatal("expected validation error")
	}
	for _, key := range []string{"mcp.max_stream_replies", "mcp.max_stream_seconds", "mcp.max_result_bytes"} {
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error lacks %s: %v", key, err)
		}
	}
}
