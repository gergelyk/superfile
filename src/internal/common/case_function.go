package common

import (
	"strings"
	"unicode"
)

type NameCase int

const (
	SnakeCase NameCase = iota
	CamelCase
	UpperCase
	DashedCase
)

// NameCases lists all supported cases in the order they are offered to the user
var NameCases = []NameCase{SnakeCase, CamelCase, UpperCase, DashedCase}

func (c NameCase) String() string {
	switch c {
	case SnakeCase:
		return "snake_case"
	case CamelCase:
		return "CamelCase"
	case UpperCase:
		return "UPPER_CASE"
	case DashedCase:
		return "dashed-name"
	}
	return ""
}

// SplitWords splits a name into words. Characters that are not letters or digits
// separate words, and so do CamelCase boundaries.
func SplitWords(s string) []string {
	runes := []rune(s)
	var words []string
	start := -1
	for i := range runes {
		switch {
		case !isWordRune(runes[i]):
			if start >= 0 {
				words = append(words, string(runes[start:i]))
				start = -1
			}
		case start < 0:
			start = i
		case isWordStart(runes, i):
			words = append(words, string(runes[start:i]))
			start = i
		}
	}
	if start >= 0 {
		words = append(words, string(runes[start:]))
	}
	return words
}

// ConvertCase rewrites a name without extension in the given case.
// It returns the name unchanged if it contains no words.
func ConvertCase(name string, c NameCase) string {
	words := SplitWords(name)
	if len(words) == 0 {
		return name
	}
	switch c {
	case SnakeCase:
		return strings.ToLower(strings.Join(words, "_"))
	case DashedCase:
		return strings.ToLower(strings.Join(words, "-"))
	case UpperCase:
		return strings.ToUpper(strings.Join(words, "_"))
	case CamelCase:
		var sb strings.Builder
		for _, w := range words {
			runes := []rune(strings.ToLower(w))
			runes[0] = unicode.ToUpper(runes[0])
			sb.WriteString(string(runes))
		}
		return sb.String()
	}
	return name
}
