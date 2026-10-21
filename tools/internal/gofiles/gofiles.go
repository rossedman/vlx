// Package gofiles walks the hand-written and generated Go source files of
// this repository for the repo check tools in tools/.
package gofiles

import (
	"bufio"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// File is one parsed, non-test Go source file.
type File struct {
	// Path is the file's path as found on disk, e.g. "internal/cli/app.go".
	Path string
	// AST is the parsed file, with comments.
	AST *ast.File
	// Generated reports whether the file carries the standard
	// "// Code generated ... DO NOT EDIT." header.
	Generated bool
}

// Walk parses every non-test .go file matched by patterns. A pattern
// ending in "/..." matches its directory recursively; any other pattern
// names a single directory. Hidden directories, testdata, vendor,
// node_modules and site are skipped. Files are returned sorted by path.
func Walk(patterns []string) ([]File, error) {
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	seen := map[string]bool{}
	var files []File
	add := func(path string) error {
		if seen[path] || !isSource(path) {
			return nil
		}
		seen[path] = true
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files = append(files, File{Path: filepath.ToSlash(path), AST: f, Generated: ast.IsGenerated(f)})
		return nil
	}
	for _, p := range patterns {
		if dir, ok := strings.CutSuffix(p, "/..."); ok {
			err := filepath.WalkDir(filepath.Clean(dir), func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					if path != filepath.Clean(dir) && skipDir(d.Name()) {
						return filepath.SkipDir
					}
					return nil
				}
				return add(path)
			})
			if err != nil {
				return nil, err
			}
			continue
		}
		entries, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() {
				if err := add(filepath.Join(p, e.Name())); err != nil {
					return nil, err
				}
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// isSource reports whether path is a Go file that is not a test.
func isSource(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

// skipDir reports whether a directory named name holds no source to check.
func skipDir(name string) bool {
	switch name {
	case "testdata", "vendor", "node_modules", "site":
		return true
	}
	return strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

// ModulePath returns the module path declared in the go.mod file at
// gomod.
func ModulePath(gomod string) (string, error) {
	f, err := os.Open(gomod)
	if err != nil {
		return "", err
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(s.Text()), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	if err := s.Err(); err != nil {
		return "", err
	}
	return "", errors.New(gomod + ": no module directive")
}
