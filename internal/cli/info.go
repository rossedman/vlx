package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/client"
	"github.com/rossedman/vlx/internal/sanitize"
)

func newInfoCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "info SERVICE",
		GroupID: "explore",
		Short:   "Show a service's vendor, product, version and interfaces",
		Long: `Ask a service for org.varlink.service.GetInfo: who made it, which version it
is and which interfaces it implements. This is always read-only.`,
		Example: `  vlx info hostname
  vlx info /run/systemd/io.systemd.Manager -o json`,
		Args:              usageArgs(cobra.ExactArgs(1)),
		ValidArgsFunction: completeService(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := a.resolve(args[0])
			if err != nil {
				return err
			}
			info, err := client.Info(cmd.Context(), path, a.clientOptions())
			if err != nil {
				return err
			}
			if a.json() {
				return writeJSON(a.out, struct {
					Path string `json:"path"`
					*infoJSON
				}{path, (*infoJSON)(info)})
			}
			color := a.color()
			label := func(s string) string {
				if color {
					return mutedStyle.Render(fmt.Sprintf("%-11s", s))
				}
				return fmt.Sprintf("%-11s", s)
			}
			fmt.Fprintln(a.out, label("socket")+path)
			for _, kv := range [][2]string{{"vendor", info.Vendor}, {"product", info.Product}, {"version", info.Version}, {"url", info.URL}} {
				if kv[1] != "" {
					fmt.Fprintln(a.out, label(kv[0])+sanitize.Line(kv[1]))
				}
			}
			for i, iface := range info.Interfaces {
				l := ""
				if i == 0 {
					l = "interfaces"
				}
				fmt.Fprintln(a.out, label(l)+sanitize.Line(iface))
			}
			return nil
		},
	}
}
