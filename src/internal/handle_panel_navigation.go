package internal

import (
	"log/slog"

	"github.com/yorukot/superfile/src/internal/common"
)

// Pinned directory
func (m *model) pinnedDirectory() {
	panel := m.getFocusedFilePanel()
	err := m.sidebarModel.TogglePinnedDirectory(panel.Location)
	if err != nil {
		slog.Error("Error while toggling pinned directory", "error", err)
	}
}

// Focus on sidebar
func (m *model) focusOnSideBar() {
	if common.Config.SidebarWidth == 0 {
		return
	}
	if m.focusPanel == sidebarFocus {
		m.focusPanel = nonePanelFocus
		m.getFocusedFilePanel().IsFocused = true
	} else {
		m.focusPanel = sidebarFocus
		m.getFocusedFilePanel().IsFocused = false
	}
}

// Focus on processbar
func (m *model) focusOnProcessBar() {
	if !m.toggleFooter {
		return
	}

	if m.focusPanel == processBarFocus {
		m.focusPanel = nonePanelFocus
		m.getFocusedFilePanel().IsFocused = true
	} else {
		m.focusPanel = processBarFocus
		m.getFocusedFilePanel().IsFocused = false
	}
}

// focus on metadata
func (m *model) focusOnMetadata() {
	if !m.toggleFooter {
		return
	}

	if m.focusPanel == metadataFocus {
		m.focusPanel = nonePanelFocus
		m.getFocusedFilePanel().IsFocused = true
	} else {
		m.focusPanel = metadataFocus
		m.getFocusedFilePanel().IsFocused = false
	}
}

// focusFilePanelAt focuses the file panel under the given screen cell, if any.
// Clicks are ignored while a modal is open.
func (m *model) focusFilePanelAt(x, y int) {
	if m.anyModalOpen() || y >= m.mainPanelHeight+common.BorderPadding {
		return
	}
	if m.sidebarVisible() {
		x -= m.sidebarModel.GetWidth()
	}
	index := m.fileModel.PanelIndexAtX(x)
	if index < 0 {
		return
	}
	m.focusPanel = nonePanelFocus
	m.fileModel.FocusPanel(index)
}

func (m *model) anyModalOpen() bool {
	return m.spfError.IsOpen() || m.helpMenu.IsOpen() || m.promptModal.IsOpen() ||
		m.zoxideModal.IsOpen() || m.editorPicker.IsOpen() || m.sortModal.IsOpen() ||
		m.notifyModel.IsOpen() || m.typingModal.open
}
