package render

import (
	"strings"

	"github.com/rossedman/vlx/internal/sanitize"
)

var idlKeywords = map[string]bool{"interface": true, "type": true, "method": true, "error": true}

var idlBuiltins = map[string]bool{"bool": true, "int": true, "float": true, "string": true, "object": true, "any": true}

// IDL highlights Varlink IDL text line by line: comments, keywords, the
// declared name, field names and types. With color false it only
// sanitises.
func IDL(src string, color bool) string {
	src = strings.ReplaceAll(sanitize.Text(src), "\t", "    ")
	if !color {
		return src
	}
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		lines[i] = idlLine(line)
	}
	return strings.Join(lines, "\n")
}

func idlLine(line string) string {
	trimmed := strings.TrimLeft(line, " \t")
	indent := line[:len(line)-len(trimmed)]
	if strings.HasPrefix(trimmed, "#") {
		return indent + CommentStyle.Render(trimmed)
	}
	var b strings.Builder
	b.WriteString(indent)
	// Tokenise into identifiers and punctuation.
	expectName := false
	i := 0
	for i < len(trimmed) {
		c := trimmed[i]
		switch {
		case isIdent(c):
			j := i
			for j < len(trimmed) && (isIdent(trimmed[j]) || trimmed[j] == '.') {
				j++
			}
			word := trimmed[i:j]
			rest := strings.TrimLeft(trimmed[j:], " \t")
			switch {
			case idlKeywords[word] && i == 0:
				b.WriteString(KeywordStyle.Render(word))
				expectName = true
			case expectName:
				b.WriteString(NameStyle.Render(word))
				expectName = false
			case strings.HasPrefix(rest, ":"):
				b.WriteString(KeyStyle.Render(word))
			case idlBuiltins[word]:
				b.WriteString(TypeStyle.Render(word))
			default:
				// Named types and enum values.
				b.WriteString(TypeStyle.Bold(true).Render(word))
			}
			i = j
		case c == '#':
			b.WriteString(CommentStyle.Render(trimmed[i:]))
			i = len(trimmed)
		default:
			j := i
			for j < len(trimmed) && !isIdent(trimmed[j]) && trimmed[j] != '#' {
				j++
			}
			b.WriteString(PunctStyle.Render(trimmed[i:j]))
			i = j
		}
	}
	return b.String()
}

func isIdent(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_'
}
