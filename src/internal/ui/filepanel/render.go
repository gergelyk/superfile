package filepanel

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/config/icon"
	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/internal/ui/rendering"
	"github.com/yorukot/superfile/src/internal/ui/sortmodel"
)

/*
- TODO: Write File Panel Specific unit test
  - Individual panel resizes
  - Footer content of filepanel changes due to resizing
  - i Only mode icons remains on smaller
  - ii Other things that change too
  - Other panels like clipboard and metadata's content changes too on resize
*/
// PanelNumberLabel returns the label shown on the panel's top border for the
// panel at the given zero-based index: "1".."9", then "0" for the tenth panel.
func PanelNumberLabel(index int) string {
	const labelCount = 10
	if index < 0 || index >= labelCount {
		return ""
	}
	return strconv.Itoa((index + 1) % labelCount)
}

func (m *Model) Render(focused bool, index int) string {
	r := ui.FilePanelRenderer(m.height, m.width, focused)
	r.SetBorderTopRightLabel(PanelNumberLabel(index))
	r.SetBorderTitleCentered(m.getFreeSpaceText())

	m.renderTopBar(r)
	m.renderSearchBar(r)
	m.renderFooter(r, m.SelectedCount())
	if m.NeedRenderHeaders() {
		m.renderColumnHeaders(r)
	}
	m.renderFileEntries(r)
	return r.Render()
}

func (m *Model) renderTopBar(r *rendering.Renderer) {
	// TODO - Add ansitruncate left in renderer and remove truncation here
	truncatedPath := common.TruncateTextBeginning(m.Location, m.GetContentWidth()-common.InnerPadding, "...")
	r.AddLines(common.FilePanelTopDirectoryIcon + common.FilePanelTopPathStyle.Render(truncatedPath))
	r.AddSection()
}

// getFreeSpaceText returns text like "12.3 GiB free" for the filesystem of the panel's location.
func (m *Model) getFreeSpaceText() string {
	if m.freeSpaceLocation == m.Location && time.Since(m.freeSpaceUpdated) < freeSpaceRefreshInterval {
		return m.freeSpaceText
	}
	m.freeSpaceLocation = m.Location
	m.freeSpaceUpdated = time.Now()
	m.freeSpaceText = ""
	if free, ok := getFreeSpace(m.Location); ok {
		m.freeSpaceText = common.FormatFileSize(int64(min(free, math.MaxInt64))) + " free" //nolint:gosec // clamped
	}
	return m.freeSpaceText
}

func (m *Model) renderSearchBar(r *rendering.Renderer) {
	r.AddLines(" " + m.SearchBar.View())
}

// TODO : Unit test this
func (m *Model) renderFooter(r *rendering.Renderer, selectedCount uint) {
	sortLabel, sortIcon := m.getSortInfo()
	modeLabel, modeIcon := m.getPanelModeInfo(selectedCount)
	cursorStr := m.getCursorString()

	if common.Config.Nerdfont {
		sortLabel = sortIcon + icon.Space + sortLabel
		modeLabel = modeIcon + icon.Space + modeLabel
	} else {
		// TODO : Figure out if we can set icon.Space to " " if nerdfont is false
		// That would simplify code
		sortLabel = sortIcon + " " + sortLabel
	}

	if common.Config.ShowPanelFooterInfo {
		r.SetBorderInfoItems(sortLabel, modeLabel, cursorStr)
		if r.AreInfoItemsTruncated() {
			r.SetBorderInfoItems(sortIcon, modeIcon, cursorStr)
		}
	} else {
		r.SetBorderInfoItems(cursorStr)
	}
}

func (m *Model) renderColumnHeaders(r *rendering.Renderer) {
	var builder strings.Builder
	for _, column := range m.columns {
		builder.WriteString(column.RenderHeader())
	}
	r.AddLines(builder.String())
}

func (m *Model) renderFileEntries(r *rendering.Renderer) {
	if m.Empty() {
		r.AddLines(common.FilePanelNoneText)
		return
	}
	end := min(m.renderIndex+m.PanelElementHeight(), m.ElemCount())

	for itemIndex := m.renderIndex; itemIndex < end; itemIndex++ {
		if m.Renaming && itemIndex == m.GetCursor() {
			r.AddLines(m.renderRenameInput())
			continue
		}
		var builder strings.Builder
		for _, column := range m.columns {
			colData := column.Render(itemIndex)
			builder.WriteString(colData)
		}
		r.AddLines(builder.String())
	}
}

func (m *Model) getSortInfo() (string, string) {
	iconStr := icon.SortAsc
	if m.SortReversed {
		iconStr = icon.SortDesc
	}
	return sortmodel.SortOptionsShortStr[m.SortKind], iconStr
}

func (m *Model) getPanelModeInfo(selectedCount uint) (string, string) {
	switch m.PanelMode {
	case BrowserMode:
		return "Browser", icon.Browser
	case SelectMode:
		return "Select" + icon.Space + fmt.Sprintf("(%d)", selectedCount), icon.Select
	default:
		return "", ""
	}
}

func (m *Model) getCursorString() string {
	cursor := m.GetCursor()
	if !m.Empty() {
		cursor++ // Convert to 1-based
	}
	return fmt.Sprintf("%d/%d", cursor, m.ElemCount())
}

func (m *Model) renderSelectBox(isSelected bool) string {
	if !common.Config.ShowSelectIcons || !common.Config.Nerdfont || m.PanelMode != SelectMode {
		return ""
	}

	if m.IsFocused {
		if isSelected {
			return common.CheckboxCheckedFocused
		}
		return common.CheckboxEmptyFocused
	}
	if isSelected {
		return common.CheckboxChecked
	}
	return common.CheckboxEmpty
}

// Checks whether a panel needs re-render due to being invalid or due to directory change
func (m *Model) NeedsReRender() bool {
	if !m.EmptyOrInvalid() {
		return filepath.Dir(m.GetFirstElement().Location) != m.Location
	}
	return true
}

// renameInputPrefix is what a normal row has before the name, so that the edited name stays in place
func (m *Model) renameInputPrefix() string {
	elem := m.GetFocusedItem()
	isLink := elem.Info != nil && elem.Info.Mode()&os.ModeSymlink != 0
	iconStyle := common.GetElementIcon(elem.Name, elem.Directory && !isLink, isLink, common.Config.Nerdfont)
	prefix := common.FilePanelCursorStyle.Render(emptyCursor+" ") + m.renderSelectBox(m.CheckSelected(elem.Location))
	if common.SuffixMode == common.SuffixLeft {
		extWidth, _ := m.visibleSuffixLayout()
		prefix += common.ModalStyle.Render(strings.Repeat(" ", extWidth+1))
	}
	return prefix + common.StringColorRender(lipgloss.Color(iconStyle.Color), common.ModalBGColor).
		Background(common.ModalBGColor).Render(iconStyle.Icon+"  ")
}

func (m *Model) renderRenameInput() string {
	prefix := m.renameInputPrefix()
	m.Rename.SetWidth(max(1, m.GetContentWidth()-ansi.StringWidth(prefix)-1))
	if !m.RenameSelected {
		return prefix + m.Rename.View()
	}
	runes := []rune(m.Rename.Value())
	end := min(m.RenameSelectEnd, len(runes))
	return prefix + common.RenameSelectionStyle.Render(string(runes[:end])) +
		common.ModalStyle.Render(string(runes[end:]))
}
