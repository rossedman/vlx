//go:build !unix

package config

import "io/fs"

// checkOwner does nothing on non-Unix platforms, where file ownership is
// not checked.
func checkOwner(string, fs.FileInfo) error { return nil }
