package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/version"
)

// newVersionCommand builds `vlx version`, which prints the version with commit, build date,
// Go, platform and sdvarlink details; --short prints only the version.
func newVersionCommand(a *app) *cobra.Command {
	var short bool
	cmd := &cobra.Command{
		Use:     "version",
		GroupID: "other",
		Short:   "Print version and build information",
		Args:    usageArgs(cobra.NoArgs),
		RunE: func(*cobra.Command, []string) error {
			info := version.Get()
			if short {
				fmt.Fprintln(a.out, info.Version)
				return nil
			}
			if a.json() {
				return writeJSON(a.out, info)
			}
			fmt.Fprintf(a.out, "vlx %s\n", info.Version)
			for _, kv := range [][2]string{
				{"commit", info.Commit},
				{"built", info.Date},
				{"go", info.GoVersion},
				{"platform", info.Platform},
				{"sdvarlink", info.Sdvarlink},
			} {
				if kv[1] != "" {
					fmt.Fprintf(a.out, "  %-10s %s\n", kv[0], kv[1])
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&short, "short", false, "print only the version number")
	return cmd
}
