package cli

import (
	"fmt"

	"github.com/spf13/cobra"
	"go.yaml.in/yaml/v3"
)

// newConfigShowCommand builds `vlx config show`, which prints the effective settings as
// YAML, or as JSON with -o json.
func newConfigShowCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the effective configuration",
		Long:  "Print the configuration after applying defaults, the file, environment variables and flags.",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(*cobra.Command, []string) error {
			if a.json() {
				return writeJSON(a.out, map[string]any{
					"allow_writes":   a.cfg.AllowWrites,
					"timeout":        a.cfg.Timeout.String(),
					"max_frame_size": a.cfg.MaxFrameSize,
					"max_replies":    a.cfg.MaxReplies,
					"preview":        a.cfg.Preview,
					"search_paths":   a.cfg.SearchPaths,
					"redact":         a.cfg.Redact,
					"safety":         map[string][]string{"read": a.cfg.Safety.Read, "write": a.cfg.Safety.Write, "danger": a.cfg.Safety.Danger},
					"file":           a.cfg.File,
				})
			}
			src := "defaults (no config file)"
			if a.cfg.File != "" {
				src = a.cfg.File
			}
			fmt.Fprintf(a.out, "# effective configuration, from %s\n", src)
			out := map[string]any{
				"allow_writes":   a.cfg.AllowWrites,
				"timeout":        a.cfg.Timeout.String(),
				"max_frame_size": a.cfg.MaxFrameSize,
				"max_replies":    a.cfg.MaxReplies,
				"preview":        a.cfg.Preview,
				"search_paths":   a.cfg.SearchPaths,
				"redact":         a.cfg.Redact,
				"safety":         map[string][]string{"read": a.cfg.Safety.Read, "write": a.cfg.Safety.Write, "danger": a.cfg.Safety.Danger},
			}
			enc := yaml.NewEncoder(a.out)
			enc.SetIndent(2)
			return enc.Encode(out)
		},
	}
}
