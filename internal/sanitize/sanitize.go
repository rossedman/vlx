// Package sanitize makes strings that come from a Varlink service safe to
// print to a terminal.
//
// Interface names, doc comments, error names and reply values are all
// controlled by the service. Printed raw, a malicious or buggy service could
// embed escape sequences that move the cursor, rewrite the screen, change the
// window title or write to the clipboard (OSC 52). Everything vlx renders from
// a service goes through this package first.
package sanitize

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Line returns s with every control character removed and newlines and tabs
// turned into spaces, for single-line contexts such as list items.
func Line(s string) string {
	return clean(s, false)
}

// Text returns s with every control character removed except newline and
// tab, for multi-line contexts such as doc comments.
func Text(s string) string {
	return clean(s, true)
}

// clean drops control characters, carriage returns and bidi overrides and
// replaces invalid UTF-8; newlines and tabs are kept when multiline, else
// turned into spaces.
func clean(s string, multiline bool) string {
	if isClean(s, multiline) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size
		switch {
		case r == utf8.RuneError && size == 1:
			b.WriteRune(unicode.ReplacementChar)
		case r == '\n' || r == '\t':
			if multiline {
				b.WriteRune(r)
			} else {
				b.WriteByte(' ')
			}
		case r == '\r':
			// Dropped: a bare carriage return can overwrite a line.
		case unicode.IsControl(r), isBidiOverride(r):
			// C0, DEL and C1 controls (ESC, CSI, OSC, ...) and bidi
			// overrides that can disguise text.
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isClean is the fast path for plain ASCII that clean would leave alone; it
// returns false for any non-ASCII byte.
func isClean(s string, multiline bool) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf {
			return false // let the slow path check multi-byte runes
		}
		if c < 0x20 || c == 0x7f {
			if multiline && (c == '\n' || c == '\t') {
				continue
			}
			return false
		}
	}
	return true
}

// isBidiOverride reports whether r is a bidi embedding, override or
// isolate control, which can make text display out of order.
func isBidiOverride(r rune) bool {
	return (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069)
}
