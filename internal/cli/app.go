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

	in     io.Reader
	out    io.Writer
	errOut io.Writer
}

func newApp() *app {
	v := viper.New()
	config.SetDefaults(v)
	return &app{v: v, in: os.Stdin, out: os.Stdout, errOut: os.Stderr}
}

// load resolves the configuration once flags are parsed.
func (a *app) load() error {
	cfg, err := config.Load(a.v, a.configFile)
	if err != nil {
		return usageError(err)
	}
	a.cfg = cfg
	if cfg.NoColor || !a.outIsTTY() {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
	return nil
}

func isTTY(f any) bool {
	file, ok := f.(*os.File)
	if !ok {
		return false
	}
	return isatty.IsTerminal(file.Fd()) || isatty.IsCygwinTerminal(file.Fd())
}

func (a *app) outIsTTY() bool { return isTTY(a.out) }
func (a *app) inIsTTY() bool  { return isTTY(a.in) }

func (a *app) json() bool { return a.cfg.Output == "json" }

// color reports whether human output may use colour.
func (a *app) color() bool { return !a.cfg.NoColor && a.outIsTTY() }

func (a *app) clientOptions() client.Options {
	return client.Options{Timeout: a.cfg.Timeout, MaxFrameSize: a.cfg.MaxFrameSize}
}

func (a *app) discoveryOptions() discovery.Options {
	extra := append(append([]string(nil), a.cfg.SearchPaths...), a.search...)
	return discovery.Options{Extra: extra}
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

func containsSlash(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return true
		}
	}
	return false
}
