// Package cli defines vlx's commands, one Cobra command per file.
package cli

import (
	"io"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-isatty"
	"github.com/muesli/termenv"
	"github.com/spf13/viper"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/discovery"
)

// app is the state shared by every command: configuration and I/O.
type app struct {
	v          *viper.Viper
	cfg        *config.Config
	configFile string
	search     []string
	noPreview  bool

	in     io.Reader
	out    io.Writer
	errOut io.Writer

	// tty overrides terminal detection for in, out and errOut; tests set
	// it. nil means ask the operating system.
	tty func(stream any) bool
	// dirs replaces the default search directories; tests set it.
	dirs []string
}

// newApp returns an app wired to the process's stdin, stdout and stderr, with config defaults set.
func newApp() *app {
	v := viper.New()
	config.SetDefaults(v)
	return &app{v: v, in: os.Stdin, out: os.Stdout, errOut: os.Stderr}
}

// load resolves the configuration once flags are parsed.
func (a *app) load() error {
	return a.loadConfig(a.configFile)
}

// loadIfExists is load for commands that create the config file: a
// --config path that does not exist yet is not an error.
func (a *app) loadIfExists() error {
	file := a.configFile
	if _, err := os.Stat(file); err != nil {
		file = ""
	}
	return a.loadConfig(file)
}

// loadConfig reads file (or only defaults and the environment when empty) into a.cfg,
// applies --no-preview, and turns colour off for NO_COLOR or a non-terminal stdout.
func (a *app) loadConfig(file string) error {
	cfg, err := config.Load(a.v, file)
	if err != nil {
		return usageError(err)
	}
	a.cfg = cfg
	if a.noPreview {
		cfg.Preview = false
	}
	if cfg.NoColor || !a.outIsTTY() {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	return nil
}

// isTTY reports whether f is an *os.File attached to a terminal, Cygwin's included.
func isTTY(f any) bool {
	file, ok := f.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}

// isTTY reports whether stream is a terminal, asking a.tty first when a test set it.
func (a *app) isTTY(stream any) bool {
	if a.tty != nil {
		return a.tty(stream)
	}
	return isTTY(stream)
}

// outIsTTY reports whether stdout is a terminal.
func (a *app) outIsTTY() bool { return a.isTTY(a.out) }

// inIsTTY reports whether stdin is a terminal.
func (a *app) inIsTTY() bool { return a.isTTY(a.in) }

// errIsTTY reports whether stderr is a terminal.
func (a *app) errIsTTY() bool { return a.isTTY(a.errOut) }

// json reports whether -o json was asked for.
func (a *app) json() bool { return a.cfg.Output == "json" }

// color reports whether human output may use colour.
func (a *app) color() bool { return !a.cfg.NoColor && a.outIsTTY() }

// clientOptions returns the timeout and frame-size limit from the config for a client call.
func (a *app) clientOptions() client.Options {
	return client.Options{Timeout: a.cfg.Timeout, MaxFrameSize: a.cfg.MaxFrameSize}
}

// discoveryOptions returns the socket search settings: the default (or test) directories
// plus the config's search_paths and any --search directories.
func (a *app) discoveryOptions() discovery.Options {
	extra := append(append([]string(nil), a.cfg.SearchPaths...), a.search...)
	return discovery.Options{Dirs: a.dirs, Extra: extra}
}

// resolve turns a SERVICE argument into a socket path.
func (a *app) resolve(arg string) (string, error) {
	var sockets []discovery.Socket
	if !containsSlash(arg) {
		sockets = discovery.Discover(a.discoveryOptions())
	}
	p, err := discovery.Resolve(arg, sockets)
	if err != nil {
		return "", &ExitError{Code: ExitNotFound, Err: err}
	}
	return p, nil
}

// containsSlash reports whether s has a '/', meaning it is a path rather than a name to discover.
func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}
