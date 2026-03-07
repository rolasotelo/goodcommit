package pluginutil

import (
	"unicode"
	"unicode/utf8"
)

func UppercaseFirstRune(s string) string {
	return mapFirstRune(s, unicode.ToUpper)
}

func LowercaseFirstRune(s string) string {
	return mapFirstRune(s, unicode.ToLower)
}

func mapFirstRune(s string, mapper func(rune) rune) string {
	if s == "" {
		return ""
	}

	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size <= 1 {
		return s
	}

	return string(mapper(r)) + s[size:]
}
