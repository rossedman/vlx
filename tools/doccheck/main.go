// Command doccheck fails when a function, method or package in hand-written
// Go code has no doc comment. Test files and generated files are skipped.
//
//	go run ./tools/doccheck ./...
package main

import (
	"fmt"
	"go/ast"
	"io"
	"os"
	"path"

	"github.com/rossedman/vlx/tools/internal/gofiles"
)

// main runs doccheck on the patterns given as arguments.
func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run reports every missing doc comment under patterns on out and returns
// the exit code: 0 when none are missing, 1 when some are, 2 on error.
func run(patterns []string, out, errOut io.Writer) int {
	files, err := gofiles.Walk(patterns)
	if err != nil {
		fmt.Fprintln(errOut, "doccheck:", err)
		return 2
	}
	missing := check(files, out)
	if missing > 0 {
		fmt.Fprintf(errOut, "doccheck: %d missing doc comment(s)\n", missing)
		return 1
	}
	return 0
}

// check writes one line per missing doc comment in files and returns how
// many it found. A package needs a package comment in at least one of its
// files.
func check(files []gofiles.File, out io.Writer) int {
	missing := 0
	pkgDoc := map[string]bool{}
	var dirs []string
	for _, f := range files {
		if f.Generated {
			continue
		}
		dir := path.Dir(f.Path)
		if _, ok := pkgDoc[dir]; !ok {
			pkgDoc[dir] = false
			dirs = append(dirs, dir)
		}
		if f.AST.Doc != nil {
			pkgDoc[dir] = true
		}
		for _, fn := range funcs(f) {
			fmt.Fprintf(out, "%s: %s has no doc comment\n", f.Path, fn)
			missing++
		}
	}
	for _, dir := range dirs {
		if !pkgDoc[dir] {
			fmt.Fprintf(out, "%s: package has no package comment\n", dir)
			missing++
		}
	}
	return missing
}

// funcs returns the names of the functions and methods in f that have no
// doc comment, with methods written as "Type.Method".
func funcs(f gofiles.File) []string {
	var names []string
	for _, decl := range f.AST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Doc != nil {
			continue
		}
		name := fn.Name.Name
		if fn.Recv != nil && len(fn.Recv.List) > 0 {
			name = recvType(fn.Recv.List[0].Type) + "." + name
		}
		names = append(names, name)
	}
	return names
}

// recvType returns the bare type name of a method receiver, dropping
// pointers and type parameters.
func recvType(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvType(t.X)
	case *ast.IndexExpr:
		return recvType(t.X)
	case *ast.IndexListExpr:
		return recvType(t.X)
	case *ast.Ident:
		return t.Name
	}
	return "?"
}
