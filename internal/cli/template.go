package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/render"
	"github.com/rossedman/vlx/internal/schema"
)

func newTemplateCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "template SERVICE METHOD",
		Aliases: []string{"skeleton"},
		GroupID: "call",
		Short:   "Print a JSON parameter template for a method",
		Long: `Print a JSON object with every required parameter of METHOD set to a typed
placeholder, ready to edit and pass to 'vlx call'. Optional parameters are
listed on stderr.

METHOD may be qualified (io.systemd.Hostname.Describe) or bare (Describe)
when only one interface of the service has it.`,
		Example: `  vlx template resolve ResolveHostname > params.json
  vlx call resolve ResolveHostname -f params.json`,
		Args:              usageArgs(cobra.ExactArgs(2)),
		ValidArgsFunction: completeService(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := a.resolve(args[0])
			if err != nil {
				return err
			}
			svc, err := client.Introspect(cmd.Context(), path, a.clientOptions())
			if err != nil {
				return err
			}
			iface, m, err := svc.FindMethod(args[1])
			if err != nil {
				return &ExitError{Code: ExitNotFound, Err: err}
			}
			text, optional := schema.Skeleton(iface.Parsed, m.In)
			fmt.Fprintln(a.out, render.JSON([]byte(text), render.JSONOptions{Color: a.color()}))
			if len(optional) > 0 && !a.json() {
				fmt.Fprintln(a.errOut, "# optional parameters:")
				for _, o := range optional {
					fmt.Fprintln(a.errOut, "#   "+o)
				}
			}
			return nil
		},
	}
}
