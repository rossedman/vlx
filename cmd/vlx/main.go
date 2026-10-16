// Command vlx is a terminal explorer for Varlink services.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/rossedman/vlx/internal/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

// run executes vlx with args and returns the exit code, reporting errors
// (with a hint, when there is one) on stderr.
func run(args []string, stderr io.Writer) int {
	cmd := cli.NewRootCommand()
	cmd.SetArgs(args)
	err := cmd.Execute()
	if err != nil && !cli.IsSilent(err) {
		fmt.Fprintln(stderr, "vlx:", err)
		if hint := cli.Hint(err); hint != "" {
			fmt.Fprintln(stderr, "hint:", hint)
		}
	}
	return cli.ExitCode(err)
}
