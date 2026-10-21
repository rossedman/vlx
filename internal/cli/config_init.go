package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/rossedman/vlx/internal/config"
)

// newConfigInitCommand builds `vlx config init`, which writes the commented starter config
// with mode 0600 and refuses to overwrite an existing file without --force.
func newConfigInitCommand(a *app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Write a commented starter config file",
		Long: `Write a commented config file with the default settings to the config path
(or --config). The file is created with mode 0600: vlx refuses config files
that others can write, because the config can enable writes.`,
		Args: usageArgs(cobra.NoArgs),
		PersistentPreRunE: func(*cobra.Command, []string) error {
			return a.loadIfExists()
		},
		RunE: func(*cobra.Command, []string) error {
			p := a.configFile
			if p == "" {
				p = config.DefaultPath()
			}
			if p == "" {
				return errors.New("cannot determine the config directory; pass --config")
			}
			if _, err := os.Stat(p); err == nil && !force {
				return usageError(fmt.Errorf("%s already exists; use --force to overwrite it", p))
			} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
				return err
			}
			if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
				return err
			}
			if err := os.WriteFile(p, []byte(config.Template), 0o600); err != nil {
				return err
			}
			if err := os.Chmod(p, 0o600); err != nil {
				return err
			}
			fmt.Fprintln(a.errOut, "wrote", p)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing file")
	return cmd
}
