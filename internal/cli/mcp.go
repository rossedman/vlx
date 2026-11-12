package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/mcpserver"
	"github.com/rossedman/vlx/internal/version"
)

// newMCPCommand builds `vlx mcp`, which serves vlx to an AI agent over the
// Model Context Protocol on stdin and stdout.
func newMCPCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "mcp",
		GroupID: "other",
		Short:   "Serve vlx to an AI agent over the Model Context Protocol",
		Long: `Run an MCP server on stdin and stdout, so an AI agent such as Claude Code can
discover, inspect and call the Varlink services on this host.

The server offers four tools: list_services, describe, method_schema and call.
Each interface's IDL is also a resource, varlink://<service>/<interface>.
Everything a service returns is sanitised, redacted and labelled as data before
the agent sees it.

Only READ methods are sent. WRITE and DANGER methods are refused, with or
without --allow-writes, because this version cannot ask you to confirm them.
Streams stop after mcp.max_stream_replies replies or mcp.max_stream_seconds
seconds, and one result is at most mcp.max_result_bytes; see 'vlx config show'.

Register it with your agent, for example:

  claude mcp add vlx -- vlx mcp`,
		Example: `  vlx mcp
  claude mcp add vlx -- vlx mcp
  claude mcp add vlx-demo -- vlx mcp --search "$XDG_RUNTIME_DIR/vlx"`,
		Args: usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if a.cfg.AllowWrites {
				fmt.Fprintln(a.errOut, "vlx mcp: --allow-writes has no effect yet; WRITE and DANGER methods are refused.")
			}
			srv := mcpserver.New(a.mcpOptions())
			ctx, stop := signalContext(cmd.Context())
			defer stop()
			t := &mcp.IOTransport{Reader: io.NopCloser(a.in), Writer: nopWriteCloser{a.out}}
			if err := srv.Run(ctx, t); err != nil && !isDisconnect(err) && ctx.Err() == nil {
				return fmt.Errorf("mcp server: %w", err)
			}
			return nil
		},
	}
}

// mcpOptions builds the server options from the resolved configuration.
func (a *app) mcpOptions() mcpserver.Options {
	return mcpserver.Options{
		Discovery:   a.discoveryOptions(),
		Client:      a.clientOptions(),
		Safety:      a.cfg.Safety,
		AllowWrites: a.cfg.AllowWrites,
		Redact:      a.cfg.Redact,
		MCP:         a.cfg.MCP,
		Version:     version.Get().Version,
	}
}

// isDisconnect reports whether err only means the client went away.
func isDisconnect(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrClosedPipe) || errors.Is(err, mcp.ErrConnectionClosed)
}

// nopWriteCloser adds a Close that does nothing, so the transport never
// closes the process's stdout.
type nopWriteCloser struct{ io.Writer }

// Close does nothing.
func (nopWriteCloser) Close() error { return nil }
