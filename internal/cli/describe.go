package cli

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/render"
)

func newDescribeCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "describe SERVICE [INTERFACE]",
		Aliases: []string{"introspect", "idl"},
		GroupID: "explore",
		Short:   "Print the IDL of a service's interfaces",
		Long: `Print the Varlink IDL of every interface a service implements, or of one
interface, as the service describes it. Output is highlighted on a terminal.`,
		Example: `  vlx describe hostname
  vlx describe resolve io.systemd.Resolve
  vlx describe hostname -o json`,
		Args:              cobra.RangeArgs(1, 2),
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
			ifaces := svc.Interfaces
			if len(args) == 2 {
				i := svc.Interface(args[1])
				if i == nil {
					return &ExitError{Code: ExitNotFound, Err: fmt.Errorf("%s does not implement %s", path, args[1])}
				}
				ifaces = []*client.Interface{i}
			}
			if a.json() {
				type out struct {
					Interface   string `json:"interface"`
					Description string `json:"description,omitempty"`
					Error       string `json:"error,omitempty"`
				}
				var res []out
				for _, i := range ifaces {
					o := out{Interface: i.Name, Description: i.Description}
					if i.Err != nil {
						o.Error = i.Err.Error()
					}
					res = append(res, o)
				}
				return writeJSON(a.out, res)
			}
			var errs []error
			for n, i := range ifaces {
				if n > 0 {
					fmt.Fprintln(a.out)
				}
				if i.Description == "" && i.Err != nil {
					errs = append(errs, fmt.Errorf("%s: %w", i.Name, i.Err))
					continue
				}
				fmt.Fprintln(a.out, render.IDL(trimNL(i.Description), a.color()))
			}
			return errors.Join(errs...)
		},
	}
}

func trimNL(s string) string {
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == ' ') {
		s = s[:len(s)-1]
	}
	return s
}
