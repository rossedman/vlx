package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/discovery"
)

type listEntry struct {
	discovery.Socket
	Product    string   `json:"product,omitempty"`
	Version    string   `json:"version,omitempty"`
	Interfaces []string `json:"interfaces,omitempty"`
	Error      string   `json:"error,omitempty"`
}

// newListCommand builds `vlx list`, which prints the discovered sockets as a table or JSON;
// --probe also asks each one for its GetInfo.
func newListCommand(a *app) *cobra.Command {
	var probe bool
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		GroupID: "explore",
		Short:   "List the Varlink sockets on this host",
		Long: `List the Varlink sockets found in the standard locations (/run/varlink/registry,
/run/systemd and its subdirectories, /run/udev and $XDG_RUNTIME_DIR/systemd),
plus any --search directories and search_paths from the config.

Listing only reads directories. With --probe, vlx also connects to each socket
and asks for its GetInfo, which is read-only.`,
		Example: `  vlx list
  vlx list --probe
  vlx list -o json | jq -r '.[].path'`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			sockets := discovery.Discover(a.discoveryOptions())
			entries := make([]listEntry, len(sockets))
			for i, s := range sockets {
				entries[i] = listEntry{Socket: s}
			}
			if probe {
				probeAll(cmd.Context(), a, entries)
			}
			if a.json() {
				return writeJSON(a.out, entries)
			}
			if len(entries) == 0 {
				fmt.Fprintln(a.errOut, "No Varlink sockets found. Try 'vlx demo', or --search DIR.")
				return nil
			}
			header := []string{"NAME", "PATH", "ACCESS"}
			if probe {
				header = append(header, "PRODUCT", "INTERFACES")
			}
			t := newTable(a.out, header...)
			for _, e := range entries {
				access := "yes"
				if !e.Accessible {
					access = "root"
				}
				row := []string{e.Name, e.Path, access}
				if probe {
					if e.Error != "" {
						row = append(row, "✗ "+e.Error, "")
					} else {
						row = append(row, strings.TrimSpace(e.Product+" "+e.Version), fmt.Sprint(len(e.Interfaces)))
					}
				}
				t.add(row...)
			}
			return t.flush(a.color(), nameStyle)
		},
	}
	cmd.Flags().BoolVar(&probe, "probe", false, "connect to each socket and ask for its info")
	return cmd
}

// probeAll fills in each entry's GetInfo, eight sockets at a time, recording failures in the
// entry's Error.
func probeAll(ctx context.Context, a *app, entries []listEntry) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for i := range entries {
		wg.Add(1)
		go func(e *listEntry) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info, err := client.Info(ctx, e.Path, a.clientOptions())
			if err != nil {
				e.Error = shortErr(err)
				return
			}
			e.Product, e.Version, e.Interfaces = info.Product, info.Version, info.Interfaces
		}(&entries[i])
	}
	wg.Wait()
}

// shortErr keeps only the last part of a wrapped error message, to fit a table cell.
func shortErr(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 && i < len(s)-2 {
		s = s[i+2:]
	}
	return s
}
