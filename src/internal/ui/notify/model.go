package notify

import (
	"charm.land/lipgloss/v2"

	"github.com/yorukot/superfile/src/internal/common"
)

type Model struct {
	open          bool
	title         string
	content       string
	confirmAction ConfirmActionType
}

func New(open bool, title string, content string, confirmAction ConfirmActionType) Model {
	return Model{
		open:          open,
		title:         title,
		content:       content,
		confirmAction: confirmAction,
	}
}

func (m *Model) GetTitle() string {
	return m.title
}

func (m *Model) GetContent() string {
	return m.content
}

func (m *Model) IsOpen() bool {
	return m.open
}

func (m *Model) Open() {
	m.open = true
}

func (m *Model) Close() {
	m.open = false
}

func (m *Model) GetConfirmAction() ConfirmActionType {
	return m.confirmAction
}

// TODO: Remove code duplication with typineModalRender
func (m *Model) Render() string {
	var inputKeysText string
	switch m.confirmAction { //nolint:exhaustive // other actions share the default buttons
	case SymlinkAction:
		// Three buttons need a narrower gap than the default to fit in one line
		gap := lipgloss.NewStyle().Background(common.ModalBGColor).Render("  ")
		inputKeysText = common.ModalConfirm.Render(" (r) Relative ") + gap +
			common.ModalConfirm.Render(" (a) Absolute ") + gap + common.ModalCancelInputText
	case NoAction, BulkRenameRetryAction:
		inputKeysText = common.ModalOkayInputText
	default:
		inputKeysText = common.ModalConfirmInputText + common.ModalInputSpacingText + common.ModalCancelInputText
	}
	return common.ModalBorderStyle(common.ModalHeight, common.ModalWidth).
		Render(m.title + "\n\n" + m.content + "\n\n" + inputKeysText)
}
