package filemodel

import "charm.land/lipgloss/v2"

func (m *Model) Render() string {
	f := make([]string, m.PanelCount()+1)
	f[0] = m.GetFilePreviewRender()
	for i, filePanel := range m.FilePanels {
		f[i+1] = filePanel.Render(filePanel.IsFocused, i)
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, f...)
}

func (m *Model) GetFilePreviewRender() string {
	if !m.FilePreview.IsOpen() {
		return ""
	}
	// Check if width and height have been synced yet
	if m.FilePreview.GetContentHeight() == m.Height &&
		m.FilePreview.GetContentWidth() == m.ExpectedPreviewWidth {
		if m.FilePreview.IsLoading() {
			return m.FilePreview.RenderText(FilePreviewLoadingText)
		}
		return m.FilePreview.GetContent()
	}

	// Placeholder resizing text till they get synced
	return m.FilePreview.RenderTextWithDimension(
		FilePreviewResizingText, m.Height, m.ExpectedPreviewWidth)
}
