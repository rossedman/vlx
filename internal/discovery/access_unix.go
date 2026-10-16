//go:build unix

package discovery

import "golang.org/x/sys/unix"

// accessible reports whether the process may connect to the socket, which
// requires write permission on it.
func accessible(path string) bool {
	return unix.Access(path, unix.W_OK) == nil
}
