// Package version holds build metadata, set by GoReleaser through -ldflags.
package version

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// Set at build time:
//
//	-X github.com/rossedman/vlx/internal/version.Version=1.2.3
var (
	Version = "dev"
	Commit  = ""
	Date    = ""
)

// Info is the build metadata printed by `vlx version`.
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit,omitempty"`
	Date      string `json:"date,omitempty"`
	GoVersion string `json:"goVersion"`
	Platform  string `json:"platform"`
	Sdvarlink string `json:"sdvarlink,omitempty"`
}

// Get returns the build metadata, filling gaps from the module build info
// for binaries built with `go install`.
func Get() Info {
	info := Info{
		Version:   Version,
		Commit:    Commit,
		Date:      Date,
		GoVersion: runtime.Version(),
		Platform:  runtime.GOOS + "/" + runtime.GOARCH,
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		if info.Version == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			info.Version = bi.Main.Version
		}
		for _, dep := range bi.Deps {
			if dep.Path == "github.com/rossedman/sdvarlink" {
				info.Sdvarlink = dep.Version
			}
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				if info.Commit == "" && len(s.Value) >= 7 {
					info.Commit = s.Value[:7]
				}
			case "vcs.time":
				if info.Date == "" {
					info.Date = s.Value
				}
			}
		}
	}
	return info
}

// String is the one-line form used by `vlx --version`.
func (i Info) String() string {
	s := i.Version
	if i.Commit != "" {
		s += " (" + i.Commit + ")"
	}
	return fmt.Sprintf("%s %s", s, i.Platform)
}
