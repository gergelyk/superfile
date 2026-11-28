package internal

import (
	"log/slog"

	tea "charm.land/bubbletea/v2"

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
func (m *model) focusOnProcessBar() tea.Cmd {
	cmd := m.peekFooter()
	// Pressing the key again keeps the focus here. Use next_file_panel to go back to the file panel.
	m.setRegionFocus(processBarFocus)
	return cmd
}

// focus on metadata
func (m *model) focusOnMetadata() tea.Cmd {
	cmd := m.peekFooter()
	// Pressing the key again keeps the focus here. Use next_file_panel to go back to the file panel.
	m.setRegionFocus(metadataFocus)
	return cmd
}

// peekFooter shows a hidden footer, until the focus leaves it. It does not change the saved setting.
func (m *model) peekFooter() tea.Cmd {
	if m.toggleFooter {
		return nil
	}
	m.toggleFooter = true
	m.footerPeek = true
	m.setHeightValues()
	return m.updateComponentDimensions()
}

// endFooterPeek hides the footer shown by peekFooter once the focus is outside of it
func (m *model) endFooterPeek() tea.Cmd {
	if !m.footerPeek || m.focusPanel == processBarFocus || m.focusPanel == metadataFocus {
		return nil
	}
	m.footerPeek = false
	m.toggleFooter = false
	m.setHeightValues()
	return m.updateComponentDimensions()
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
