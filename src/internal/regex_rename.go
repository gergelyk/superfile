package internal

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/yorukot/superfile/src/internal/archive"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui/notify"
)

const (
	regexRenameMaxRows  = 12
	regexRenameMinWidth = 60
	regexRenameFields   = 3
	checkboxField       = 2
)

// regexRenameState holds the dialog that renames items with a regular expression
type regexRenameState struct {
	dir     string
	paths   []string
	pattern textinput.Model
	replace textinput.Model
	field   int // 0 - pattern, 1 - replacement, 2 - the suffixes checkbox
	// suffixes makes the expression apply to the extension too, not only to the name before it
	suffixes bool

	names    []string
	changes  []renameChange
	problems []string
	err      string
}

// regexRenamedNames applies the regexp to every name, leaving the extension out unless suffixes is set.
// Unchanged names are returned as they are.
func regexRenamedNames(paths []string, pattern, replacement string, suffixes bool) ([]string, error) {
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = filepath.Base(p)
	}
	if pattern == "" {
		return names, nil
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return names, err
	}
	for i, n := range names {
		end := len(n)
		if !suffixes {
			end = renameSelectionEnd(n, isDirectory(paths[i]))
			// renameSelectionEnd counts runes
			end = len(string([]rune(n)[:end]))
		}
		names[i] = re.ReplaceAllString(n[:end], replacement) + n[end:]
	}
	return names, nil
}

func (m *model) openRegexRename() {
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
	if archive.IsLocation(panel.Location) {
		m.showNotify("Cannot rename", "Regex rename is not supported inside archives", notify.NoAction)
		return
	}
	st := &regexRenameState{dir: panel.Location, paths: paths}
	st.pattern = common.GeneratePromptTextInput()
	st.pattern.Placeholder = "regular expression"
	st.pattern.SetValue("(.*)")
	st.replace = common.GeneratePromptTextInput()
	st.replace.Placeholder = "replacement, e.g. ${1}_x"
	st.replace.SetValue("${1}")
	st.pattern.Focus()
	st.update()
	m.regexRename = st
	m.firstTextInput = true
}

// focused returns the focused text input, or nil if the checkbox is focused
func (st *regexRenameState) focused() *textinput.Model {
	switch st.field {
	case 0:
		return &st.pattern
	case 1:
		return &st.replace
	}
	return nil
}

func (st *regexRenameState) switchField(delta int) {
	if in := st.focused(); in != nil {
		in.Blur()
	}
	st.field = (st.field + delta + regexRenameFields) % regexRenameFields
	if in := st.focused(); in != nil {
		in.Focus()
	}
}

// update recomputes the preview after the inputs have changed
func (st *regexRenameState) update() {
	var err error
	st.names, err = regexRenamedNames(st.paths, st.pattern.Value(), st.replace.Value(), st.suffixes)
	st.err, st.changes, st.problems = "", nil, nil
	if err != nil {
		st.err = err.Error()
		return
	}
	st.changes, st.problems = planBulkRename(st.dir, st.paths, st.names)
}

func (m *model) regexRenameKey(msg string) tea.Cmd {
	st := m.regexRename
	switch {
	case msg == "esc", msg == "ctrl+c":
		m.regexRename = nil
	case isSpaceKey(msg) && st.field == checkboxField:
		st.suffixes = !st.suffixes
		st.update()
	case slices.Contains(common.Hotkeys.ConfirmTyping, msg):
		if st.err != "" || len(st.problems) > 0 {
			return nil
		}
		m.regexRename = nil
		if len(st.changes) == 0 {
			return nil
		}
		m.bulkRename = &bulkRenameState{dir: st.dir, paths: st.paths, changes: st.changes}
		m.applyBulkRename()
	case msg == "tab", msg == "down":
		st.switchField(1)
	case msg == "shift+tab", msg == "up":
		st.switchField(-1)
	}
	return nil
}

func (st *regexRenameState) render(maxWidth int) (string, int, int) {
	width := regexRenameMinWidth
	oldWidth := 0
	for _, p := range st.paths {
		oldWidth = max(oldWidth, utf8.RuneCountInString(filepath.Base(p)))
	}
	rows := make([]string, 0, len(st.paths))
	for i, p := range st.paths {
		old := filepath.Base(p)
		line := old + strings.Repeat(" ", oldWidth-utf8.RuneCountInString(old)) + "  ->  " + st.names[i]
		rows = append(rows, line)
		width = max(width, utf8.RuneCountInString(line)+common.BorderPadding+common.InnerPadding)
	}
	width = min(width, maxWidth)
	textWidth := width - common.BorderPadding - common.InnerPadding

	st.pattern.SetWidth(textWidth - len("Find     "))
	st.replace.SetWidth(textWidth - len("Replace  "))

	var lines []string
	lines = append(lines, common.ModalTitleStyle.Render(" Rename with regex"), "")
	lines = append(lines, common.ModalStyle.Render(" Find     ")+st.pattern.View())
	lines = append(lines, common.ModalStyle.Render(" Replace  ")+st.replace.View())
	box := "[ ]"
	if st.suffixes {
		box = "[x]"
	}
	boxStyle := common.ModalStyle
	if st.field == checkboxField {
		boxStyle = common.ModalTitleStyle
	}
	lines = append(lines, boxStyle.Render(" "+box+" Process suffixes (Space to toggle)"), "")
	status := fmt.Sprintf(" %d of %d will be renamed", len(st.changes), len(st.paths))
	style := common.ModalStyle
	switch {
	case st.err != "":
		status, style = " Invalid expression: "+st.err, common.ProcessErrorStyle
	case len(st.problems) > 0:
		status, style = " "+st.problems[0], common.ProcessErrorStyle
	}
	lines = append(lines, style.Render(common.TruncateText(status, textWidth, "...")), "")
	for i, r := range rows {
		if i == regexRenameMaxRows {
			lines = append(lines, common.ModalStyle.Render(fmt.Sprintf(" ... %d more", len(rows)-i)))
			break
		}
		lines = append(lines, common.ModalStyle.Render(common.TruncateText(" "+r, textWidth, "...")))
	}
	hint := common.ModalStyle.Render(" Tab: next field, Enter: rename, Esc: cancel")
	lines = append(lines, "", hint)

	height := len(lines)
	bottom := common.GenerateFooterBorder(fmt.Sprintf("%d items", len(st.paths)), width-common.BorderPadding)
	out := common.SortOptionsModalBorderStyle(height, width, bottom).Render(strings.Join(lines, "\n"))
	return out, width, lipgloss.Height(out)
}
