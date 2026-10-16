package cli

import (
	"errors"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/tui"
)

func newExploreCommand(a *app) *cobra.Command {
	return &cobra.Command{
		Use:     "explore [SERVICE]",
		Aliases: []string{"x", "ui"},
		GroupID: "explore",
		Short:   "Open the interactive explorer (the default command)",
		Long: `Open the interactive explorer. With SERVICE, vlx connects to it straight
away; without, it lists the Varlink sockets it finds and lets you pick one.

Press ? inside the explorer for every key binding.`,
		Example: `  vlx explore
  vlx explore /run/systemd/io.systemd.Hostname
  vlx explore resolve --allow-writes`,
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completeService(a),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExplore(cmd, a, args)
		},
	}
}

func runExplore(_ *cobra.Command, a *app, args []string) error {
	return a.runTUI(args, "")
}

// runTUI starts the explorer, optionally at the service named by args[0]
// or at target.
func (a *app) runTUI(args []string, note string, target ...string) error {
	if !a.outIsTTY() || !a.inIsTTY() {
		return usageError(errors.New("the explorer needs an interactive terminal; for scripts use 'vlx list', 'vlx describe' or 'vlx call'"))
	}
	opts := tui.Options{Config: a.cfg, Discovery: a.discoveryOptions(), Note: note}
	switch {
	case len(target) > 0:
		opts.Target = target[0]
	case len(args) == 1:
		p, err := a.resolve(args[0])
		if err != nil {
			return err
		}
		opts.Target = p
	}
	m := tui.New(opts)
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}
