package common

import (
	"strings"
	"unicode"
)

type NameCase int

const (
	SnakeCase NameCase = iota
	PascalCase
	UpperCase
	KebabCase
	FlatCase
	NaturalCase
	PascalSnakeCase
	// NumberedCase replaces names with consecutive zero padded numbers. It is applied to a list, see ConvertCase.
	NumberedCase
)

// NameCases lists all supported cases in the order they are offered to the user
var NameCases = []NameCase{SnakeCase, PascalCase, UpperCase, KebabCase, FlatCase, NaturalCase, PascalSnakeCase, NumberedCase}

func (c NameCase) String() string {
	switch c {
	case SnakeCase:
		return "snake_case"
	case PascalCase:
		return "PascalCase"
	case UpperCase:
		return "UPPER_CASE"
	case KebabCase:
		return "kebab-case"
	case FlatCase:
		return "flatcase"
	case NaturalCase:
		return "Natural Case"
	case PascalSnakeCase:
		return "Pascal_Snake_Case"
	case NumberedCase:
		return "numbered"
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
	case KebabCase:
		return strings.ToLower(strings.Join(words, "-"))
	case FlatCase:
		return strings.ToLower(strings.Join(words, ""))
	case UpperCase:
		return strings.ToUpper(strings.Join(words, "_"))
	case PascalCase:
		return strings.Join(capitalizeAll(words), "")
	case NaturalCase:
		return strings.Join(capitalizeAll(words), " ")
	case PascalSnakeCase:
		return strings.Join(capitalizeAll(words), "_")
	}
	return name
}

func capitalizeAll(words []string) []string {
	res := make([]string, len(words))
	for i, w := range words {
		runes := []rune(strings.ToLower(w))
		runes[0] = unicode.ToUpper(runes[0])
		res[i] = string(runes)
	}
	return res
}
