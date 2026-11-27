// Package editorpicker is a small modal for choosing the editor to open an item with.
package editorpicker

import (
	"fmt"
	"strings"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
)

const minWidth = 30

type Model struct {
	open   bool
	Title  string
	Items  []string
	Cursor int
	Width  int
	Height int
}

func (m *Model) Open(title string, items []string) {
	m.open = true
	m.Title = title
	m.Items = items
	m.Cursor = 0
	m.Width = minWidth
	for _, it := range items {
		m.Width = max(m.Width, len([]rune(it))+common.BorderPadding+common.InnerPadding+4) //nolint:mnd // number prefix
	}
	m.Height = len(items) + 2 //nolint:mnd // title and spacer
}

func (m *Model) Close()       { m.open = false }
func (m *Model) IsOpen() bool { return m.open }

func (m *Model) ListUp() {
	if m.Cursor > 0 {
		m.Cursor--
	} else {
		m.Cursor = len(m.Items) - 1
	}
}

func (m *Model) ListDown() {
	if m.Cursor < len(m.Items)-1 {
		m.Cursor++
	} else {
		m.Cursor = 0
	}
}

// SelectNumber moves the cursor to the 1-based position n. It reports whether n is valid.
func (m *Model) SelectNumber(n int) bool {
	if n < 1 || n > len(m.Items) {
		return false
	}
	m.Cursor = n - 1
	return true
}

func (m *Model) Render() string {
	var sb strings.Builder
	sb.WriteString(common.ModalTitleStyle.Render(" "+m.Title) + "\n\n")
	for i, it := range m.Items {
		cursor := " "
		if i == m.Cursor {
			cursor = common.FilePanelCursorStyle.Render(icon.Cursor)
		}
		sb.WriteString(cursor + common.ModalStyle.Render(fmt.Sprintf(" %d. %s", i+1, it)) + "\n")
	}
	bottomBorder := common.GenerateFooterBorder(
		fmt.Sprintf("%d/%d", m.Cursor+1, len(m.Items)), m.Width-common.BorderPadding)
	return common.SortOptionsModalBorderStyle(m.Height, m.Width, bottomBorder).Render(sb.String())
}
