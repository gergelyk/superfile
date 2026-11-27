package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

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

// caseRenamedNames returns the new names for all items. Numbered names depend on the item position.
func caseRenamedNames(paths []string, c common.NameCase) []string {
	names := make([]string, len(paths))
	digits := len(strconv.Itoa(len(paths)))
	for i, p := range paths {
		name := filepath.Base(p)
		isDir := isDirectory(p)
		if c != common.NumberedCase {
			names[i] = caseRenamedName(name, isDir, c)
			continue
		}
		ext := name[renameSelectionEnd(name, isDir):]
		names[i] = fmt.Sprintf("%0*d", digits, i+1) + ext
	}
	return names
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
	labelWidth := 0
	for _, c := range common.NameCases {
		labelWidth = max(labelWidth, utf8.RuneCountInString(c.String()))
	}
	labels := make([]string, len(common.NameCases))
	for i, c := range common.NameCases {
		labels[i] = fmt.Sprintf("%-*s  %s", labelWidth, c.String(), caseRenamedNames(paths, c)[0])
	}
	m.pendingCasePaths = paths
	m.editorPicker.Open("Rename to", labels)
	if len(paths) > 1 {
		previews := make([][]string, len(common.NameCases))
		for i, c := range common.NameCases {
			width := 0
			for _, p := range paths {
				width = max(width, utf8.RuneCountInString(filepath.Base(p)))
			}
			newNames := caseRenamedNames(paths, c)
			for j, p := range paths {
				old := filepath.Base(p)
				pad := strings.Repeat(" ", width-utf8.RuneCountInString(old))
				previews[i] = append(previews[i], old+pad+"  ->  "+newNames[j])
			}
		}
		m.editorPicker.SetPreviews(previews)
	}
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
	newNames := caseRenamedNames(paths, c)
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
