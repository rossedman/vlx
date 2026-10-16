//go:build !unix

package config

import "io/fs"

func checkOwner(string, fs.FileInfo) error { return nil }
