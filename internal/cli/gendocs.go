package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/cobra/doc"
)

func newGenDocsCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "gen-docs DIR",
		Short:  "Generate the Markdown command reference into DIR",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := args[0]
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			root := cmd.Root()
			root.DisableAutoGenTag = true
			prepender := func(filename string) string {
				name := strings.ReplaceAll(strings.TrimSuffix(filepath.Base(filename), ".md"), "_", " ")
				return fmt.Sprintf("---\ndescription: Reference for the '%s' command.\n---\n\n", name)
			}
			link := func(name string) string { return name }
			return doc.GenMarkdownTreeCustom(root, dir, prepender, link)
		},
	}
}
