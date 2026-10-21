package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/demo"
)

// newDemoCommand builds `vlx demo`, which starts the in-memory demo service and opens the
// explorer on it, or with --serve only runs the service until interrupted.
func newDemoCommand(a *app) *cobra.Command {
	var serve bool
	var socket string
	cmd := &cobra.Command{
		Use:     "demo",
		GroupID: "explore",
		Short:   "Explore a built-in sample service (no systemd needed)",
		Long: `Start vlx's demo Varlink service and open the explorer on it. The service
listens on $XDG_RUNTIME_DIR/vlx/io.vlx.demo, so other vlx commands find it as
"demo", or on a private temporary socket when that is not possible. The demo has two small interfaces, io.vlx.demo.Greeter and
io.vlx.demo.Inventory, with streaming methods, typed errors, a redacted secret
and methods of every safety class. It keeps all state in memory.

Add --allow-writes to try the confirmation dialogs. Use --serve to only run
the service, for example to try 'vlx call demo ...' from another terminal.`,
		Example: `  vlx demo
  vlx demo --allow-writes
  vlx demo --serve`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			var (
				srv *demo.Server
				err error
			)
			if socket != "" {
				srv, err = demo.Listen(socket)
			} else {
				srv, err = demo.ListenUser()
			}
			if err != nil {
				return fmt.Errorf("start demo service: %w", err)
			}
			defer srv.Close()
			if !serve {
				return a.runTUI(nil, "demo service", srv.Path())
			}
			name := srv.Path()
			if name == demo.UserSocketPath() {
				name = "demo" // discoverable by name
			}
			fmt.Fprintf(a.errOut, "Demo service listening on %s\n\nTry, in another terminal:\n  vlx explore %s\n  vlx call %s GetGreeting '{\"name\":\"you\"}'\n\nPress Ctrl+C to stop.\n",
				srv.Path(), name, name)
			fmt.Fprintln(a.out, srv.Path())
			ctx, stop := signalContext(cmd.Context())
			defer stop()
			<-ctx.Done()
			return nil
		},
	}
	cmd.Flags().BoolVar(&serve, "serve", false, "only run the service and print its socket path")
	cmd.Flags().StringVar(&socket, "socket", "", "listen on this path instead of a temporary one")
	return cmd
}
