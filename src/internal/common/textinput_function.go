package common

import (
	"unicode"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

// wordBackwardPos returns the cursor position at the start of the previous word.
// Any character that is not a letter or digit separates words.
func wordBackwardPos(runes []rune, pos int) int {
	pos = min(pos, len(runes))
	for pos > 0 && !isWordRune(runes[pos-1]) {
		pos--
	}
	for pos > 0 && isWordRune(runes[pos-1]) {
		pos--
	}
	return pos
}

// wordForwardPos returns the cursor position at the end of the next word.
func wordForwardPos(runes []rune, pos int) int {
	for pos < len(runes) && !isWordRune(runes[pos]) {
		pos++
	}
	for pos < len(runes) && isWordRune(runes[pos]) {
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
