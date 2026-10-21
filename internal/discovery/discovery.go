// Package discovery finds Varlink sockets on the host and resolves the
// SERVICE arguments users type into socket paths.
//
// Discovery only lists directories: it never connects to anything, never
// follows symlinked directories and only descends one level into each
// search directory.
package discovery

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultDirs are the directories searched for sockets, one level deep.
func DefaultDirs() []string {
	dirs := []string{
		"/run/varlink/registry",
		"/run/systemd",
		"/run/systemd/resolve",
		"/run/systemd/userdb",
		"/run/systemd/machine",
		"/run/systemd/netif",
		"/run/systemd/journal",
		"/run/systemd/oom",
		"/run/systemd/report",
		"/run/udev",
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" && filepath.IsAbs(xdg) {
		dirs = append(dirs, filepath.Join(xdg, "systemd"), filepath.Join(xdg, "vlx"))
	}
	return dirs
}

// Socket is one discovered Unix socket.
type Socket struct {
	// Path is the socket path as found (possibly a symlink in a registry).
	Path string `json:"path"`
	// Name is the base name, e.g. "io.systemd.Hostname".
	Name string `json:"name"`
	// Mode is the permission bits of the socket.
	Mode fs.FileMode `json:"-"`
	// Accessible reports whether the current user can connect (has write
	// permission on the socket), as far as permission bits tell.
	Accessible bool `json:"accessible"`
}

// Options configures Discover.
type Options struct {
	// Dirs to search; DefaultDirs() when nil.
	Dirs []string
	// Extra directories appended to Dirs (from config and flags).
	Extra []string
}

// Discover lists the sockets in the search directories, de-duplicated by
// resolved path and sorted by name. Missing or unreadable directories are
// skipped.
func Discover(opts Options) []Socket {
	dirs := opts.Dirs
	if dirs == nil {
		dirs = DefaultDirs()
	}
	dirs = append(append([]string(nil), dirs...), opts.Extra...)

	seen := map[string]bool{}
	var out []Socket
	for _, dir := range dirs {
		// Do not descend into a search directory that is itself a symlink
		// to somewhere else; registries contain symlinked sockets, not
		// symlinked directories.
		if fi, err := os.Lstat(dir); err != nil || !fi.IsDir() {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			fi, err := os.Stat(p) // follows symlinked sockets
			if err != nil || fi.Mode().Type() != fs.ModeSocket {
				continue
			}
			key := p
			if real, err := filepath.EvalSymlinks(p); err == nil {
				key = real
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Socket{
				Path:       p,
				Name:       e.Name(),
				Mode:       fi.Mode().Perm(),
				Accessible: accessible(p),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// ShortName is a friendly label for a socket: "io.systemd.Hostname" becomes
// "Hostname", "io.systemd.Resolve.Monitor" becomes "Resolve.Monitor".
func ShortName(name string) string {
	for _, prefix := range []string{"io.systemd.", "org.varlink."} {
		if s, ok := strings.CutPrefix(name, prefix); ok && s != "" {
			return s
		}
	}
	return name
}

// ErrNotFound is returned by Resolve when nothing matches.
var ErrNotFound = errors.New("service not found")

// AmbiguousError is returned by Resolve when a fragment matches several
// sockets.
type AmbiguousError struct {
	Query      string
	Candidates []Socket
}

// Error lists every matching socket path and asks for a fuller name.
func (e *AmbiguousError) Error() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%q matches %d services:", e.Query, len(e.Candidates))
	for _, c := range e.Candidates {
		fmt.Fprintf(&b, "\n  %s", c.Path)
	}
	b.WriteString("\nuse the full name or path")
	return b.String()
}

// Resolve turns a SERVICE argument into a socket path. It accepts a path
// (anything containing '/'), the "unix:" address form, an exact socket name,
// or a case-insensitive fragment of one name.
func Resolve(arg string, sockets []Socket) (string, error) {
	if arg == "" {
		return "", fmt.Errorf("%w: empty service name", ErrNotFound)
	}
	if rest, ok := strings.CutPrefix(arg, "unix:"); ok {
		arg = rest
		if i := strings.IndexByte(arg, ';'); i >= 0 {
			arg = arg[:i] // varlink addresses may carry ";mode=..." options
		}
		return checkPath(arg)
	}
	if strings.ContainsRune(arg, '/') {
		return checkPath(arg)
	}
	for _, s := range sockets {
		if s.Name == arg {
			return s.Path, nil
		}
	}
	var exactFold, fragment []Socket
	lower := strings.ToLower(arg)
	for _, s := range sockets {
		name := strings.ToLower(s.Name)
		switch {
		case name == lower || strings.ToLower(ShortName(s.Name)) == lower:
			exactFold = append(exactFold, s)
		case strings.Contains(name, lower):
			fragment = append(fragment, s)
		}
	}
	for _, group := range [][]Socket{exactFold, fragment} {
		switch len(group) {
		case 0:
			continue
		case 1:
			return group[0].Path, nil
		default:
			return "", &AmbiguousError{Query: arg, Candidates: group}
		}
	}
	return "", fmt.Errorf("%w: no socket named %q (run 'vlx list' to see what is available)", ErrNotFound, arg)
}

// checkPath makes p absolute and confirms it exists and is a Unix socket.
func checkPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(abs)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: %s does not exist", ErrNotFound, abs)
		}
		return "", err
	}
	if fi.Mode().Type() != fs.ModeSocket {
		return "", fmt.Errorf("%w: %s is not a Unix socket", ErrNotFound, abs)
	}
	return abs, nil
}
