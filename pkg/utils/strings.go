package utils

import (
	"os"
	"unicode/utf8"
)

const PathSeparator = string(os.PathSeparator)

// TruncateBytes shortens s to at most max bytes without splitting a multi-byte rune.
func TruncateBytes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	cut := max
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}
