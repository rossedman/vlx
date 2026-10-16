package cli

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newManCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "man DIR",
		Short:  "Generate man pages into DIR",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := os.MkdirAll(args[0], 0o755); err != nil {
				return err
			}
			root := cmd.Root()
			root.DisableAutoGenTag = true
			return doc.GenManTree(root, &doc.GenManHeader{Title: "VLX", Section: "1", Source: "vlx", Manual: "vlx manual"}, args[0])
		},
	}
}
