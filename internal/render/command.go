package render

import (
	"strings"
)

// ShellQuote quotes s for POSIX shells, leaving simple words alone.
func ShellQuote(s string) string {
	if s == "" {
		return "''"
	}
	safe := true
	for _, r := range s {
		word := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("@%_-+=:,./", r)
		if !word {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// CallCommands returns the vlx and varlinkctl command lines equivalent to
// a call. params must be compact JSON.
func CallCommands(path, method, params string, more, oneway bool) (vlx, varlinkctl string) {
	var v, c []string
	v = append(v, "vlx", "call")
	c = append(c, "varlinkctl", "call")
	if more {
		v = append(v, "--more")
		c = append(c, "--more")
	}
	if oneway {
		v = append(v, "--oneway")
		c = append(c, "--oneway")
	}
	v = append(v, ShellQuote(path), ShellQuote(method))
	c = append(c, ShellQuote(path), ShellQuote(method))
	if params != "" && params != "{}" {
		v = append(v, ShellQuote(params))
		c = append(c, ShellQuote(params))
	} else {
		c = append(c, "'{}'")
	}
	return strings.Join(v, " "), strings.Join(c, " ")
}
