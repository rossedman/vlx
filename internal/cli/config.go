package cli

import "github.com/spf13/cobra"

func newConfigCommand(a *app) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		GroupID: "other",
		Short:   "Inspect or create the configuration file",
		Long: `vlx reads settings from flags, then VLX_* environment variables, then the
config file, then built-in defaults. These commands show where the file is,
what the effective settings are, and write a commented starter file.`,
		Args: cobra.NoArgs,
	}
	cmd.AddCommand(newConfigPathCommand(a), newConfigShowCommand(a), newConfigInitCommand(a))
	return cmd
}
