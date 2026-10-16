package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/config"
)

func newConfigPathCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(*cobra.Command, []string) error {
			p := a.cfg.File
			if p == "" {
				p = config.DefaultPath()
			}
			_, err := fmt.Fprintln(a.out, p)
			return err
		},
	}
}
