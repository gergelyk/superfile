package internal

import (
	"fmt"
	"image/color"
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
	caseTab             = 0
	regexTab            = 1
	renameTabs          = 2
)

// regexRenameState holds the rename dialog, with a tab for changing the case and a tab for regex
type regexRenameState struct {
	dir     string
	paths   []string
	tab     int // caseTab or regexTab
	caseIdx int // index in common.NameCases, used in the case tab
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

func (m *model) openRenameDialog() {
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
	st.pattern.SetVirtualCursor(false)
	st.pattern.Placeholder = "regular expression"
	st.pattern.SetValue("(.*)")
	st.replace = common.GeneratePromptTextInput()
	st.replace.SetVirtualCursor(false)
	st.replace.Placeholder = "replacement, e.g. ${1}_x"
	st.replace.SetValue("${1}")
	st.update()
	m.regexRename = st
	m.firstTextInput = true
}

// focused returns the focused text input, or nil if the checkbox or the case tab is focused
func (st *regexRenameState) focused() *textinput.Model {
	if st.tab != regexTab {
		return nil
	}
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

func (st *regexRenameState) switchTab(delta int) {
	if in := st.focused(); in != nil {
		in.Blur()
	}
	st.tab = (st.tab + delta + renameTabs) % renameTabs
	if in := st.focused(); in != nil {
		in.Focus()
	}
	st.update()
}

// update recomputes the preview after the inputs have changed
func (st *regexRenameState) update() {
	if st.tab == caseTab {
		st.names = caseRenamedNames(st.paths, common.NameCases[st.caseIdx])
		st.err = ""
		st.changes, st.problems = planBulkRename(st.dir, st.paths, st.names)
		return
	}
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
	case msg == "tab":
		st.switchTab(1)
	case msg == "shift+tab":
		st.switchTab(-1)
	case st.tab == caseTab && (isListUp(msg) || isListDown(msg)):
		delta := 1
		if isListUp(msg) {
			delta = -1
		}
		st.caseIdx = (st.caseIdx + delta + len(common.NameCases)) % len(common.NameCases)
		st.update()
	case isSpaceKey(msg) && st.tab == regexTab && st.field == checkboxField:
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
	case msg == "down":
		st.switchField(1)
	case msg == "up":
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

	var lines []string
	lines = append(lines, "")
	if st.tab == caseTab {
		labelWidth := 0
		for _, c := range common.NameCases {
			labelWidth = max(labelWidth, utf8.RuneCountInString(c.String()))
		}
		for i, c := range common.NameCases {
			label := fmt.Sprintf(" %-*s", labelWidth, c.String())
			if i == st.caseIdx {
				lines = append(lines, common.ModalTitleStyle.Render(">"+label))
			} else {
				lines = append(lines, common.ModalStyle.Render(" "+label))
			}
		}
		lines = append(lines, "")
	} else {
		inputWidth := width - common.BorderPadding - len(" Replace  ") - 1
		lines = append(lines, common.ModalStyle.Render(" Find     ")+
			renderRenameInput(&st.pattern, st.field == 0, inputWidth))
		lines = append(lines, common.ModalStyle.Render(" Replace  ")+
			renderRenameInput(&st.replace, st.field == 1, inputWidth))
		box := "[ ]"
		if st.suffixes {
			box = "[x]"
		}
		boxStyle := common.ModalStyle
		if st.field == checkboxField {
			boxStyle = common.ModalTitleStyle
		}
		lines = append(lines, boxStyle.Render(" "+box+" Process suffixes"), "")
	}
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

	height := len(lines)
	bottom := common.GenerateFooterBorder(
		fmt.Sprintf("Tab: Case/Regex | %d items", len(st.paths)), width-common.BorderPadding)
	out := common.ModalBorderStyleWithEdges(height, width, st.topBorder(width-common.BorderPadding), bottom).Render(strings.Join(lines, "\n"))
	return out, width, lipgloss.Height(out)
}

// topBorder returns the top border with the tab titles. The active tab is in brackets.
func (st *regexRenameState) topBorder(width int) string {
	titles := [renameTabs]string{"Case", "Regex"}
	parts := make([]string, 0, renameTabs)
	for i, t := range titles {
		if i == st.tab {
			t = "[" + t + "]"
		}
		parts = append(parts, t)
	}
	title := common.Config.BorderMiddleRight + " " + strings.Join(parts, " ") + " " + common.Config.BorderMiddleLeft
	return title + strings.Repeat(common.Config.BorderTop, max(0, width-utf8.RuneCountInString(title)))
}

// blendColors moves from towards to by the given ratio, so the result is a bit different from from
func blendColors(from, to color.Color, ratio float64) color.Color {
	const maxChannel = 0xffff
	r1, g1, b1, _ := from.RGBA()
	r2, g2, b2, _ := to.RGBA()
	mix := func(a, b uint32) uint32 {
		return uint32(float64(a)+(float64(b)-float64(a))*ratio) * 0xff / maxChannel
	}
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", mix(r1, r2), mix(g1, g2), mix(b1, b2)))
}

// renderRenameInput draws the input on its own background, the focused one is highlighted.
// The virtual cursor is off, so the focus is only shown by the colors.
func renderRenameInput(in *textinput.Model, focused bool, width int) string {
	const unfocusedMix, focusedMix = 0.12, 0.22
	style := lipgloss.NewStyle().Foreground(common.ModalFGColor).
		Background(blendColors(common.ModalBGColor, common.ModalFGColor, unfocusedMix))
	if focused {
		style = style.Foreground(common.ModalTitleStyle.GetForeground()).Bold(true).
			Background(blendColors(common.ModalBGColor, common.ModalFGColor, focusedMix))
	}
	styles := in.Styles()
	styles.Focused.Text, styles.Blurred.Text = style, style
	styles.Focused.Placeholder, styles.Blurred.Placeholder = style.Faint(true), style.Faint(true)
	in.SetStyles(styles)
	in.SetWidth(width)
	return style.Width(width).Render(in.View())
}

func isListUp(msg string) bool {
	return msg == "up" || slices.Contains(common.Hotkeys.ListUp, msg)
}

func isListDown(msg string) bool {
	return msg == "down" || slices.Contains(common.Hotkeys.ListDown, msg)
}
