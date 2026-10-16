package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/discovery"
)

// completeService completes SERVICE arguments with discovered socket names.
// It only lists directories; it never connects.
func completeService(a *app) cobra.CompletionFunc {
	return func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		if strings.Contains(toComplete, "/") {
			return nil, cobra.ShellCompDirectiveDefault
		}
		var out []string
		for _, s := range discovery.Discover(discovery.Options{Extra: a.search}) {
			out = append(out, s.Name+"\t"+s.Path)
		}
		return out, cobra.ShellCompDirectiveNoFileComp
	}
}
