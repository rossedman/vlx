//go:build unix

package config

import (
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

func checkOwner(path string, fi fs.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	uid := os.Getuid()
	if int(st.Uid) != uid && st.Uid != 0 {
		return &InsecureError{path, fmt.Sprintf("it is owned by uid %d, not by you (uid %d) or root", st.Uid, uid)}
	}
	return nil
}
