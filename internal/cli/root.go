package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/config"
	"github.com/rossedman/vlx/internal/version"
)

// NewRootCommand builds the vlx command tree.
func NewRootCommand() *cobra.Command {
	return newRootCommand(newApp())
}

// newRootCommand builds the root command around a: the persistent flags bound to viper,
// the command groups and every subcommand. Bare `vlx` opens the explorer.
func newRootCommand(a *app) *cobra.Command {
	root := &cobra.Command{
		Use:   "vlx [SERVICE]",
		Short: "Explore and call Varlink services",
		Long: `vlx is a terminal explorer for Varlink services, such as systemd's
io.systemd.* APIs. Run it with no arguments to find the sockets on this host
and browse their interfaces, methods, types and errors.

vlx is read-only by default: methods that change state are refused unless you
pass --allow-writes, and even then vlx asks before sending them.

SERVICE is a socket path, a "unix:" address, or the name (or a unique part
of the name) of a discovered socket, e.g. "hostname".`,
		Example: `  vlx                         # discover sockets and explore
  vlx hostname                # open io.systemd.Hostname directly
  vlx demo                    # try vlx with a built-in sample service
  vlx call hostname io.systemd.Hostname.Describe`,
		Version:       version.Get().String(),
		Args:          usageArgs(cobra.MaximumNArgs(1)),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return a.load()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplore(cmd, a, args)
		},
	}
	root.SetVersionTemplate("vlx {{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err)
	})

	f := root.PersistentFlags()
	f.StringVar(&a.configFile, "config", "", "config file (default $XDG_CONFIG_HOME/vlx/config.yaml)")
	f.Duration(config.KeyTimeout, 10*time.Second, "timeout for connecting and for single replies")
	f.Bool("allow-writes", false, "allow methods that change state (each still needs confirmation)")
	f.StringP(config.KeyOutput, "o", "text", "output format: text or json")
	f.Bool("no-color", false, "disable colour (also NO_COLOR=1)")
	f.Int("max-frame-size", 16<<20, "largest message accepted from a service, in bytes")
	f.StringSliceVar(&a.search, "search", nil, "extra directory to search for sockets (repeatable)")
	f.BoolVar(&a.noPreview, "no-preview", false, "in the explorer, connect to a service only when you open it")

	bind := map[string]string{
		config.KeyTimeout:      config.KeyTimeout,
		config.KeyAllowWrites:  "allow-writes",
		config.KeyOutput:       config.KeyOutput,
		config.KeyNoColor:      "no-color",
		config.KeyMaxFrameSize: "max-frame-size",
	}
	for key, flag := range bind {
		_ = a.v.BindPFlag(key, f.Lookup(flag))
	}

	_ = root.RegisterFlagCompletionFunc(config.KeyOutput, cobra.FixedCompletions([]string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp))
	root.ValidArgsFunction = completeService(a)

	root.AddGroup(
		&cobra.Group{ID: "explore", Title: "Explore:"},
		&cobra.Group{ID: "call", Title: "Call:"},
		&cobra.Group{ID: "other", Title: "Other:"},
	)
	root.AddCommand(
		newExploreCommand(a),
		newListCommand(a),
		newInfoCommand(a),
		newDescribeCommand(a),
		newTemplateCommand(a),
		newCallCommand(a),
		newDemoCommand(a),
		newConfigCommand(a),
		newVersionCommand(a),
		newManCommand(),
		newGenDocsCommand(),
	)
	root.SetHelpCommandGroupID("other")
	root.SetCompletionCommandGroupID("other")
	return root
}
