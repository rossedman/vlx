package version

import (
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	old := [3]string{Version, Commit, Date}
	defer func() { Version, Commit, Date = old[0], old[1], old[2] }()
	Version, Commit, Date = "1.2.3", "abcdef0", "2026-01-01"
	info := Get()
	if info.Version != "1.2.3" || info.Commit != "abcdef0" || info.GoVersion == "" || !strings.Contains(info.Platform, "/") {
		t.Fatalf("%+v", info)
	}
	if s := info.String(); !strings.HasPrefix(s, "1.2.3 (abcdef0) ") {
		t.Errorf("String = %q", s)
	}
	Version, Commit = "dev", ""
	if s := Get().String(); s == "" {
		t.Error("empty String")
	}
	if (Info{Version: "v", Platform: "p"}).String() != "v p" {
		t.Error("String without commit")
	}
}
