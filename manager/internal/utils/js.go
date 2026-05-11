package utils

import "strings"

// EscapeJSSingleQuotedString makes a value safe to embed inside a JavaScript/TypeScript single-quoted string literal.
// It does not add surrounding quotes.
func EscapeJSSingleQuotedString(s string) string {
	// Order matters: escape backslashes first.
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	s = strings.ReplaceAll(s, "\r\n", `\n`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\n`)
	return s
}
