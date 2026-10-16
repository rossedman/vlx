// Command vlx is a terminal explorer for Varlink services.
package main

import (
	"fmt"
	"os"

	"github.com/rossedman/vlx/internal/cli"
)

func main() {
	err := cli.NewRootCommand().Execute()
	if err != nil && !cli.IsSilent(err) {
		fmt.Fprintln(os.Stderr, "vlx:", err)
		if hint := cli.Hint(err); hint != "" {
			fmt.Fprintln(os.Stderr, "hint:", hint)
		}
	}
	os.Exit(cli.ExitCode(err))
}
