//go:build !unix

package discovery

// accessible always reports true on non-Unix platforms, which have no
// access(2) check; every socket is listed as reachable.
func accessible(string) bool { return true }
