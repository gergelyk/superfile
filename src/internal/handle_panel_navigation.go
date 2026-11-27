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

// focusRegionAt focuses the region (sidebar, file panel, process bar or metadata)
// under the given screen cell, if any. Clicks are ignored while a modal is open.
func (m *model) focusRegionAt(x, y int) {
	if m.anyModalOpen() {
		return
	}
	mainPanelEnd := m.mainPanelHeight + common.BorderPadding
	if y >= mainPanelEnd {
		if m.toggleFooter && y < m.fullHeight-1 {
			m.focusFooterAt(x)
		}
		return
	}
	if m.sidebarVisible() {
		if x < m.sidebarModel.GetWidth() {
			m.setRegionFocus(sidebarFocus)
			return
		}
		x -= m.sidebarModel.GetWidth()
	}
	index := m.fileModel.PanelIndexAtX(x)
	if index < 0 {
		return
	}
	m.setRegionFocus(nonePanelFocus)
	m.fileModel.FocusPanel(index)
}

// focusFooterAt focuses the footer component at screen column x.
// The footer is laid out as process bar, metadata and clipboard.
func (m *model) focusFooterAt(x int) {
	processBarEnd := m.processBarModel.GetWidth()
	switch {
	case x < processBarEnd:
		m.setRegionFocus(processBarFocus)
	case x < processBarEnd+m.fileMetaData.GetWidth():
		m.setRegionFocus(metadataFocus)
	}
}

// setRegionFocus moves focus to the given region. nonePanelFocus means a file panel.
func (m *model) setRegionFocus(region focusPanelType) {
	m.focusPanel = region
	m.getFocusedFilePanel().IsFocused = region == nonePanelFocus
}

func (m *model) anyModalOpen() bool {
	return m.spfError.IsOpen() || m.helpMenu.IsOpen() || m.promptModal.IsOpen() ||
		m.zoxideModal.IsOpen() || m.editorPicker.IsOpen() || m.sortModal.IsOpen() ||
		m.notifyModel.IsOpen() || m.typingModal.open || m.regexRename != nil
}
