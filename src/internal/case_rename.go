package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
)

// caseRenamedName converts the name of an item to the given case, keeping the extension.
func caseRenamedName(name string, isDir bool, c common.NameCase) string {
	stemEnd := renameSelectionEnd(name, isDir)
	runes := []rune(name)
	stem, ext := string(runes[:stemEnd]), string(runes[stemEnd:])
	if c == common.UpperCase {
		ext = strings.ToUpper(ext)
	}
	return common.ConvertCase(stem, c) + ext
}

// openCaseRenamePicker lets the user pick a case for the selected items, or for the
// focused item if nothing is selected
func (m *model) openCaseRenamePicker() {
	panel := m.getFocusedFilePanel()
	var paths []string
	if panel.SelectedCount() > 0 {
		paths = panel.GetSelectedLocationsSortedAsVisible()
	} else if !panel.Empty() {
		paths = []string{panel.GetFocusedItem().Location}
	}
	if len(paths) == 0 {
		return
	}
	first := filepath.Base(paths[0])
	firstIsDir := isDirectory(paths[0])
	labels := make([]string, len(common.NameCases))
	for i, c := range common.NameCases {
		labels[i] = fmt.Sprintf("%-12s %s", c.String(), caseRenamedName(first, firstIsDir, c))
	}
	m.pendingCasePaths = paths
	m.editorPicker.Open("Rename to", labels)
}

// closeEditorPicker closes the picker, which is used for both editors and case renames
func (m *model) closeEditorPicker() {
	m.editorPicker.Close()
	m.pendingCasePaths = nil
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (m *model) applyCaseRename(paths []string, caseIdx int) {
	if caseIdx < 0 || caseIdx >= len(common.NameCases) {
		return
	}
	c := common.NameCases[caseIdx]
	dir := filepath.Dir(paths[0])
	newNames := make([]string, len(paths))
	for i, p := range paths {
		newNames[i] = caseRenamedName(filepath.Base(p), isDirectory(p), c)
	}
	changes, problems := planBulkRename(dir, paths, newNames)
	if len(problems) > 0 {
		m.showNotify("Cannot rename", formatLimited(problems), notify.NoAction)
		return
	}
	if len(changes) == 0 {
		return
	}
	m.bulkRename = &bulkRenameState{dir: dir, paths: paths, changes: changes}
	m.applyBulkRename()
}
