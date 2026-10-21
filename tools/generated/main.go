// Command generated prints the import-path-qualified name of every
// generated Go file (one with a "// Code generated ... DO NOT EDIT." header),
// one per line, so the coverage gate can drop them from a cover profile.
//
//	go run ./tools/generated ./...
package main

import (
	"fmt"
	"io"
	"os"
	"path"

	"github.com/rossedman/vlx/tools/internal/gofiles"
)

// main runs generated on the patterns given as arguments, from the module
// root.
func main() {
	os.Exit(run("go.mod", os.Args[1:], os.Stdout, os.Stderr))
}

// run prints the generated files under patterns, qualified with the module
// path read from gomod, and returns the exit code.
func run(gomod string, patterns []string, out, errOut io.Writer) int {
	mod, err := gofiles.ModulePath(gomod)
	if err != nil {
		fmt.Fprintln(errOut, "generated:", err)
		return 2
	}
	files, err := gofiles.Walk(patterns)
	if err != nil {
		fmt.Fprintln(errOut, "generated:", err)
		return 2
	}
	for _, f := range files {
		if f.Generated {
			fmt.Fprintln(out, path.Join(mod, path.Clean(f.Path)))
		}
	}
	return 0
}
