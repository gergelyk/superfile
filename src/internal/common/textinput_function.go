package common

import (
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// isWordStart reports whether a word begins at index i. Any character that is not a
// letter or digit separates words, and so does a lower case letter or digit followed by an
// upper case letter ("fooBar"), or an acronym followed by a capitalized word ("HTTPServer").
func isWordStart(runes []rune, i int) bool {
	if !isWordRune(runes[i]) {
		return false
	}
	if i == 0 || !isWordRune(runes[i-1]) {
		return true
	}
	prev, cur := runes[i-1], runes[i]
	if unicode.IsUpper(cur) && !unicode.IsUpper(prev) {
		return true
	}
	return unicode.IsUpper(prev) && unicode.IsUpper(cur) &&
		i+1 < len(runes) && unicode.IsLower(runes[i+1])
}

// wordBackwardPos returns the cursor position at the start of the previous word.
func wordBackwardPos(runes []rune, pos int) int {
	for i := min(pos, len(runes)) - 1; i >= 0; i-- {
		if isWordStart(runes, i) {
			return i
		}
	}
	return 0
}

// wordForwardPos returns the cursor position at the end of the next word.
func wordForwardPos(runes []rune, pos int) int {
	for pos < len(runes) && !isWordRune(runes[pos]) {
		pos++
	}
	if pos < len(runes) {
		pos++
	}
	for pos < len(runes) && isWordRune(runes[pos]) && !isWordStart(runes, pos) {
		pos++
	}
	return pos
}

// UpdateTextInput updates a text input like its Update method does, except that
// ctrl/alt + left/right move the cursor by words, where punctuation also separates words.
func UpdateTextInput(ti textinput.Model, msg tea.Msg) (textinput.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		runes := []rune(ti.Value())
		switch key.String() {
		case "ctrl+left", "alt+left":
			ti.SetCursor(wordBackwardPos(runes, ti.Position()))
			return ti, nil
		case "ctrl+right", "alt+right":
			ti.SetCursor(wordForwardPos(runes, ti.Position()))
			return ti, nil
		}
	}
	return ti.Update(msg)
}
