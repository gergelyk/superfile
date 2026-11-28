package internal

import (
	"log/slog"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/metadata"
	"github.com/yorukot/superfile/src/internal/ui/notify"
	"github.com/yorukot/superfile/src/internal/ui/processbar"
	"github.com/yorukot/superfile/src/internal/ui/spferror"
)

type ModelUpdateMessage interface {
	ApplyToModel(m *model) tea.Cmd
	GetReqID() int
}

type BaseMessage struct {
	reqID int
}

func (msg BaseMessage) GetReqID() int {
	return msg.reqID
}

type PasteOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
}

func NewPasteOperationMsg(state processbar.ProcessState, reqID int) PasteOperationMsg {
	return PasteOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg PasteOperationMsg) ApplyToModel(m *model) tea.Cmd {
	if (msg.state == processbar.Failed || msg.state == processbar.Successful) && m.clipboard.IsCut() {
		m.clipboard.Reset(false)
	}
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	return nil
}

type CreateOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
	// path of the created item, it gets the cursor
	path string
}

func NewCreateOperationMsg(state processbar.ProcessState, reqID int, path string) CreateOperationMsg {
	return CreateOperationMsg{
		state: state,
		path:  path,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg CreateOperationMsg) ApplyToModel(m *model) tea.Cmd {
	if msg.state == processbar.Successful {
		m.focusItem(msg.path)
	}
	return nil
}

// focusItem moves the cursor of the focused panel to the item, if the panel shows its directory
func (m *model) focusItem(path string) {
	panel := m.getFocusedFilePanel()
	if panel.Location != filepath.Dir(path) {
		return
	}
	panel.TargetFile = filepath.Base(path)
	m.fileModel.UpdateFilePanelsIfNeeded(true)
}

type DeleteOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
}

func NewDeleteOperationMsg(state processbar.ProcessState, reqID int) DeleteOperationMsg {
	return DeleteOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg DeleteOperationMsg) ApplyToModel(m *model) tea.Cmd {
	// Remove selection
	m.getFocusedFilePanel().ResetSelected()
	return nil
}

type ProcessBarUpdateMsg struct {
	BaseMessage

	pMsg processbar.UpdateMsg
}

func (msg ProcessBarUpdateMsg) ApplyToModel(m *model) tea.Cmd {
	cmd, err := msg.pMsg.Apply(&m.processBarModel)
	if err != nil {
		slog.Error("Error applying processbar update", "error", err)
	}
	return processCmdToTeaCmd(cmd)
}

type CompressOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
	// path of the zip file, it gets the cursor
	path string
}

func NewCompressOperationMsg(state processbar.ProcessState, reqID int, path string) CompressOperationMsg {
	return CompressOperationMsg{
		state: state,
		path:  path,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg CompressOperationMsg) ApplyToModel(m *model) tea.Cmd {
	if msg.state == processbar.Successful {
		m.focusItem(msg.path)
	}
	return nil
}

type ExtractOperationMsg struct {
	BaseMessage

	state processbar.ProcessState
}

func NewExtractOperationMsg(state processbar.ProcessState, reqID int) ExtractOperationMsg {
	return ExtractOperationMsg{
		state: state,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg ExtractOperationMsg) ApplyToModel(_ *model) tea.Cmd {
	return nil
}

type MetadataMsg struct {
	BaseMessage

	meta            metadata.Metadata
	metadataFocused bool
}

func NewMetadataMsg(meta metadata.Metadata, metadataFocused bool, reqID int) MetadataMsg {
	return MetadataMsg{
		meta:            meta,
		metadataFocused: metadataFocused,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg MetadataMsg) ApplyToModel(m *model) tea.Cmd {
	m.fileMetaData.SetMetadataCache(msg.meta, msg.metadataFocused)
	selectedItem := m.getFocusedFilePanel().GetFocusedItemPtr()
	if selectedItem == nil {
		slog.Debug("Panel empty or cursor invalid. Ignoring MetadataMsg")
		return nil
	}
	if selectedItem.Location != msg.meta.GetPath() {
		slog.Debug("MetadataMsg for older files. Ignoring",
			"currentItem", selectedItem.Location, "msgItem", msg.meta.GetPath())
		return nil
	}
	if (m.focusPanel == metadataFocus) != msg.metadataFocused {
		slog.Debug("MetadataMsg for older state. Ignoring",
			"actualFocus", m.focusPanel, "msgFocus", msg.metadataFocused)
		return nil
	}
	m.fileMetaData.SetMetadata(msg.meta, msg.metadataFocused)
	return nil
}

type SelectionUsageMsg struct {
	BaseMessage

	key   string
	usage metadata.Usage
}

func NewSelectionUsageMsg(key string, usage metadata.Usage, reqID int) SelectionUsageMsg {
	return SelectionUsageMsg{
		key:   key,
		usage: usage,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg SelectionUsageMsg) ApplyToModel(m *model) tea.Cmd {
	m.fileMetaData.SetSelectionUsage(msg.key, msg.usage)
	return nil
}

type NotifyModalUpdateMsg struct {
	BaseMessage

	m notify.Model
}

func NewNotifyModalMsg(m notify.Model, reqID int) NotifyModalUpdateMsg {
	return NotifyModalUpdateMsg{
		m: m,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg NotifyModalUpdateMsg) ApplyToModel(m *model) tea.Cmd {
	m.notifyModel = msg.m
	return nil
}

type SpfErrorModalUpdateMsg struct {
	BaseMessage

	m spferror.Model
}

func NewSpfErrorModalMsg(m spferror.Model, reqID int) SpfErrorModalUpdateMsg {
	return SpfErrorModalUpdateMsg{
		m: m,
		BaseMessage: BaseMessage{
			reqID: reqID,
		},
	}
}

func (msg SpfErrorModalUpdateMsg) ApplyToModel(m *model) tea.Cmd {
	m.spfError = msg.m
	return nil
}
