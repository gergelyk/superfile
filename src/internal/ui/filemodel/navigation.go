package filemodel

import "log/slog"

func (m *Model) NextFilePanel() {
	m.MoveFocusedPanelBy(1)
}

func (m *Model) PreviousFilePanel() {
	m.MoveFocusedPanelBy(-1)
}

func (m *Model) MoveFocusedPanelBy(delta int) {
	if m.PanelCount() == 0 {
		slog.Error("Unexpected error: fileModel with 0 panels")
		return
	}
	m.GetFocusedFilePanel().IsFocused = false
	m.FocusedPanelIndex = (m.FocusedPanelIndex + delta + m.PanelCount()) % m.PanelCount()
	m.FilePanels[m.FocusedPanelIndex].IsFocused = true
}

// PanelIndexAtX returns the index of the file panel that covers screen column x,
// relative to the left edge of the file model. It returns -1 if x is on the preview or outside.
func (m *Model) PanelIndexAtX(x int) int {
	if m.FilePreview.IsOpen() {
		x -= m.ExpectedPreviewWidth
	}
	for i := range m.FilePanels {
		if x < 0 {
			return -1
		}
		if x < m.FilePanels[i].GetWidth() {
			return i
		}
		x -= m.FilePanels[i].GetWidth()
	}
	return -1
}

// FocusPanel moves focus to the file panel at the given index.
func (m *Model) FocusPanel(index int) {
	if index < 0 || index >= m.PanelCount() {
		return
	}
	m.GetFocusedFilePanel().IsFocused = false
	m.FocusedPanelIndex = index
	m.FilePanels[index].IsFocused = true
}
