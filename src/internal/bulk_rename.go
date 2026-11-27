package internal

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/internal/ui/notify"
)

const maxBulkRenameListed = 8

type renameChange struct {
	oldPath string
	newPath string
}

// bulkRenameState holds the data of a bulk rename that is in progress
type bulkRenameState struct {
	dir     string
	paths   []string
	lines   []string // names as currently edited by the user
	tmpFile string
	changes []renameChange
}

type bulkRenameEditorDoneMsg struct {
	BaseMessage

	err error
}

func (msg bulkRenameEditorDoneMsg) ApplyToModel(m *model) tea.Cmd {
	st := m.bulkRename
	if st == nil {
		return nil
	}
	data, readErr := os.ReadFile(st.tmpFile)
	_ = os.Remove(st.tmpFile)
	st.tmpFile = ""

	if err := errors.Join(msg.err, readErr); err != nil {
		slog.Error("Bulk rename editor failed", "error", err)
		m.bulkRename = nil
		m.showNotify("Bulk rename failed", err.Error(), notify.NoAction)
		return nil
	}

	lines := splitEditedLines(string(data))
	if len(lines) != len(st.paths) {
		m.bulkRename = nil
		m.showNotify("Bulk rename aborted",
			fmt.Sprintf("Expected %d lines, but got %d. Nothing was renamed.", len(st.paths), len(lines)),
			notify.NoAction)
		return nil
	}
	st.lines = lines

	changes, problems := planBulkRename(st.dir, st.paths, lines)
	if len(problems) > 0 {
		m.showNotify("Bulk rename errors - editor will reopen", formatLimited(problems), notify.BulkRenameRetryAction)
		return nil
	}
	if len(changes) == 0 {
		m.bulkRename = nil
		return nil
	}
	st.changes = changes
	descriptions := make([]string, 0, len(changes))
	for _, c := range changes {
		descriptions = append(descriptions, filepath.Base(c.oldPath)+" -> "+filepath.Base(c.newPath))
	}
	m.showNotify(fmt.Sprintf("Rename %d items?", len(changes)), formatLimited(descriptions), notify.BulkRenameAction)
	return nil
}

func (m *model) showNotify(title, content string, action notify.ConfirmActionType) {
	m.notifyModel = notify.New(true, title, content, action)
}

func formatLimited(items []string) string {
	if len(items) <= maxBulkRenameListed {
		return strings.Join(items, "\n")
	}
	shown := strings.Join(items[:maxBulkRenameListed], "\n")
	return fmt.Sprintf("%s\n... and %d more", shown, len(items)-maxBulkRenameListed)
}

// splitEditedLines splits editor output into lines, ignoring one trailing newline
func splitEditedLines(content string) []string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.TrimSuffix(content, "\n")
	return strings.Split(content, "\n")
}

// startBulkRename opens the names of the selected items in an editor
func (m *model) startBulkRename() tea.Cmd {
	panel := m.getFocusedFilePanel()
	paths := panel.GetSelectedLocationsSortedAsVisible()
	if len(paths) < 2 { //nolint:mnd // bulk rename needs multiple items
		return nil
	}
	lines := make([]string, len(paths))
	for i, p := range paths {
		lines[i] = filepath.Base(p)
	}
	m.bulkRename = &bulkRenameState{dir: panel.Location, paths: paths, lines: lines}
	return m.openBulkRenameEditor()
}

func (m *model) reopenBulkRenameEditor() tea.Cmd {
	if m.bulkRename == nil {
		return nil
	}
	return m.openBulkRenameEditor()
}

func (m *model) openBulkRenameEditor() tea.Cmd {
	st := m.bulkRename
	f, err := os.CreateTemp("", "spf-bulk-rename-*.txt")
	if err != nil {
		m.bulkRename = nil
		m.showNotify("Bulk rename failed", err.Error(), notify.NoAction)
		return nil
	}
	_, writeErr := f.WriteString(strings.Join(st.lines, "\n") + "\n")
	closeErr := f.Close()
	if err = errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(f.Name())
		m.bulkRename = nil
		m.showNotify("Bulk rename failed", err.Error(), notify.NoAction)
		return nil
	}
	st.tmpFile = f.Name()
	reqID := m.nextIoReqCnt()
	return tea.ExecProcess(editorCommand(st.tmpFile), func(err error) tea.Msg {
		return bulkRenameEditorDoneMsg{BaseMessage: BaseMessage{reqID: reqID}, err: err}
	})
}

// planBulkRename returns the renames to perform and the problems that the
// user can fix by editing the names again.
func planBulkRename(dir string, paths []string, newNames []string) ([]renameChange, []string) {
	oldNames := make(map[string]bool, len(paths))
	for _, p := range paths {
		oldNames[filepath.Base(p)] = true
	}
	counts := make(map[string]int, len(newNames))
	for _, n := range newNames {
		counts[n]++
	}

	var changes []renameChange
	var problems []string
	for i, name := range newNames {
		oldName := filepath.Base(paths[i])
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, fmt.Sprintf("Line %d: empty name", i+1))
		case name == "." || name == ".." || strings.ContainsAny(name, "/"+string(filepath.Separator)):
			problems = append(problems, fmt.Sprintf("Line %d: invalid name %q", i+1, name))
		case counts[name] > 1:
			problems = append(problems, fmt.Sprintf("Line %d: duplicate name %q", i+1, name))
		case name == oldName:
			// unchanged
		default:
			target := filepath.Join(dir, name)
			if _, err := os.Lstat(target); err == nil && !oldNames[name] {
				problems = append(problems, fmt.Sprintf("Line %d: %q already exists", i+1, name))
				continue
			}
			changes = append(changes, renameChange{oldPath: paths[i], newPath: target})
		}
	}
	return changes, problems
}

// applyBulkRename renames via temporary names, so swaps and chains of names work
func (m *model) applyBulkRename() {
	st := m.bulkRename
	m.bulkRename = nil
	if st == nil {
		return
	}
	failures := executeBulkRename(st.dir, st.changes)
	m.getFocusedFilePanel().ResetSelected()
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	if len(failures) > 0 {
		m.showNotify("Some renames failed", formatLimited(failures), notify.NoAction)
	}
}

func executeBulkRename(dir string, changes []renameChange) []string {
	var failures []string
	tmpPaths := make([]string, len(changes))
	for i, c := range changes {
		tmp := filepath.Join(dir, fmt.Sprintf(".spf-bulk-%d-%d", os.Getpid(), i))
		if err := os.Rename(c.oldPath, tmp); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", filepath.Base(c.oldPath), err))
			continue
		}
		tmpPaths[i] = tmp
	}
	for i, c := range changes {
		if tmpPaths[i] == "" {
			continue
		}
		if err := os.Rename(tmpPaths[i], c.newPath); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", filepath.Base(c.oldPath), err))
			if rbErr := os.Rename(tmpPaths[i], c.oldPath); rbErr != nil {
				slog.Error("Failed to restore name after failed bulk rename", "path", c.oldPath, "error", rbErr)
			}
		}
	}
	return failures
}

// handleRenameSelection applies selection semantics to the single item rename
// input. It returns true if the message was consumed.
func (m *model) handleRenameSelection(panel *filepanel.Model, msg tea.Msg) bool {
	if !panel.RenameSelected {
		return false
	}
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return false
	}
	runes := []rune(panel.Rename.Value())
	end := min(panel.RenameSelectEnd, len(runes))
	removeSelection := func() {
		panel.Rename.SetValue(string(runes[end:]))
		panel.Rename.SetCursor(0)
	}
	switch key.String() {
	case "backspace", "delete":
		removeSelection()
	case "left", "home":
		panel.Rename.SetCursor(0)
	case "right", "end":
		panel.Rename.SetCursor(end)
	default:
		if key.Text != "" {
			removeSelection()
			panel.RenameSelected = false
		}
		return false
	}
	panel.RenameSelected = false
	return true
}
