package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	var stderr bytes.Buffer
	if code := run([]string{"version", "-o", "json"}, &stderr); code != 0 {
		t.Fatalf("version exited %d: %s", code, stderr.String())
	}
	stderr.Reset()
	code := run([]string{"call", "/nonexistent/socket", "a.B.C"}, &stderr)
	if code != 5 || !strings.Contains(stderr.String(), "vlx:") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	stderr.Reset()
	if code := run([]string{"--no-such-flag"}, &stderr); code != 2 || !strings.Contains(stderr.String(), "unknown flag") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}
