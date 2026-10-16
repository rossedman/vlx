//go:build !unix

package discovery

func accessible(string) bool { return true }
