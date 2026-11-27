package common

import (
	"testing"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
)

func TestUpdateTextInputWordJump(t *testing.T) {
	ti := textinput.New()
	ti.Focus()
	ti.SetValue("my-file_v2.tar.gz")

	press := func(code rune) {
		ti, _ = UpdateTextInput(ti, tea.KeyPressMsg{Code: code, Mod: tea.ModCtrl})
	}
	var positions []int
	for range 5 {
		press(tea.KeyLeft)
		positions = append(positions, ti.Position())
	}
	assert.Equal(t, []int{15, 11, 8, 3, 0}, positions)

	positions = nil
	for range 4 {
		press(tea.KeyRight)
		positions = append(positions, ti.Position())
	}
	assert.Equal(t, []int{2, 7, 10, 14}, positions)
}

func TestUpdateTextInputCamelCaseJump(t *testing.T) {
	ti := textinput.New()
	ti.Focus()
	ti.SetValue("fooBarHTTPServer2")
	ti.SetCursor(0)

	var positions []int
	for range 5 {
		ti, _ = UpdateTextInput(ti, tea.KeyPressMsg{Code: tea.KeyRight, Mod: tea.ModCtrl})
		positions = append(positions, ti.Position())
	}
	assert.Equal(t, []int{3, 6, 10, 17, 17}, positions)

	positions = nil
	for range 5 {
		ti, _ = UpdateTextInput(ti, tea.KeyPressMsg{Code: tea.KeyLeft, Mod: tea.ModCtrl})
		positions = append(positions, ti.Position())
	}
	assert.Equal(t, []int{10, 6, 3, 0, 0}, positions)
}
