package metadata

import (
	"fmt"
	"os"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/internal/ui"
	"github.com/yorukot/superfile/src/pkg/cache"
)

type Model struct {
	metadata Metadata // current metadata
	cache    *cache.Cache[Metadata]

	// It tells what the metadata should have. Its used to prevent additional requests
	// if one is already underway
	expectedLocation string
	expectedFocused  bool

	// Render state
	renderIndex int
	// Permission bit (0 - 8, from the user's r to the others' x) that is selected for toggling
	selectedPermBit int

	// Cumulative size of the selected items in the focused file panel
	selectionKey   string // identifies the selection that the fields below describe, or are being computed for
	selectionReady bool
	selection      Usage

	// Model Dimensions, including borders
	width  int
	height int
}

func New() Model {
	return Model{
		cache: cache.New[Metadata](defaultCacheSize, defaultCacheExpiration),
	}
}

// Should be at least 2x2
// TODO : Validate this
func (m *Model) SetDimensions(width int, height int) {
	m.width = width
	m.height = height
}

func (m *Model) GetHeight() int {
	return m.height
}

func (m *Model) GetWidth() int {
	return m.width
}

func (m *Model) ResetRenderIfInvalid() {
	if m.renderIndex > m.maxRenderIndex() {
		m.ResetRender()
	}
}

// maxRenderIndex is the last first-visible-row that still fills the panel,
// so that scrolling stops when the last line becomes visible.
func (m *Model) maxRenderIndex() int {
	visible := max(m.height-borderSize, 1)
	return max(m.MetadataLen()-visible, 0)
}

func (m *Model) ResetRender() {
	m.renderIndex = 0
}

func (m *Model) MetadataLen() int {
	return len(m.displayData())
}

// CanEditPermissions tells whether the displayed file has permissions that can be changed
func (m *Model) CanEditPermissions() bool {
	return m.metadata.hasMode
}

// MovePermissionSelection moves the selected permission bit, without wrapping.
// The permissions line is scrolled into view if needed.
func (m *Model) MovePermissionSelection(delta int) {
	m.selectedPermBit = min(max(m.selectedPermBit+delta, 0), permBitCount-1)
	m.ScrollToPermissions()
}

// ScrollToPermissions scrolls the panel just enough to make the permissions line visible.
func (m *Model) ScrollToPermissions() {
	visible := max(m.height-borderSize, 1)
	for i, pair := range m.displayData() {
		if pair[0] != keyPermissions {
			continue
		}
		if i < m.renderIndex {
			m.renderIndex = i
		} else if i >= m.renderIndex+visible {
			m.renderIndex = i - visible + 1
		}
		m.renderIndex = min(m.renderIndex, m.maxRenderIndex())
		return
	}
}

// TogglePermission flips the selected permission bit of the displayed file on the disk.
// It returns the path of the changed file.
func (m *Model) TogglePermission() (string, error) {
	if !m.metadata.hasMode {
		return "", nil
	}
	path := m.metadata.filepath
	// Bit 0 is user's r, which is 0o400
	flag := os.FileMode(1) << (permBitCount - 1 - m.selectedPermBit)
	if err := os.Chmod(path, m.metadata.mode^flag); err != nil {
		return path, fmt.Errorf("failed to change permissions of %s: %w", path, err)
	}
	info, err := os.Lstat(path)
	if err != nil {
		return path, fmt.Errorf("failed to read permissions of %s: %w", path, err)
	}
	m.metadata.mode = info.Mode()
	m.DropMetadataIfInCache(path)
	return path, nil
}

func (m *Model) GetSelectionKey() string {
	return m.selectionKey
}

// ExpectSelection marks the selection identified by key as the one to be displayed.
// An empty key means that nothing is selected. Results for other keys are dropped.
func (m *Model) ExpectSelection(key string) {
	m.selectionKey = key
	m.selectionReady = false
	m.selection = Usage{}
}

func (m *Model) SetSelectionUsage(key string, usage Usage) {
	if key != m.selectionKey {
		return
	}
	m.selection = usage
	m.selectionReady = true
}

// displayData returns the metadata rows as they are shown. Permissions are taken from the current
// mode, the file type is split into at most two lines, and the sizes go last.
func (m *Model) displayData() [][2]string {
	if len(m.metadata.data) == 0 {
		return m.metadata.data
	}
	res := make([][2]string, 0, len(m.metadata.data)+2) //nolint:mnd // size and selection rows
	var sizeRow *[2]string
	for _, pair := range m.metadata.data {
		switch pair[0] {
		case keyPermissions:
			if m.metadata.hasMode {
				pair[1] = formatPermissions(m.metadata.mode)
			}
		case keySize:
			sizeRow = &pair
			continue
		case keyFileType:
			lines := splitFileType(pair[1], m.width/2) //nolint:mnd // value column is at least half of the width
			res = append(res, [2]string{pair[0], lines[0]})
			for _, line := range lines[1:] {
				res = append(res, [2]string{"", line})
			}
			continue
		}
		res = append(res, pair)
	}
	if sizeRow != nil {
		res = append(res, *sizeRow)
	}
	if m.selectionKey == "" {
		return res
	}
	sel := [2]string{keySelectedSize, selectionCalculatingMsg}
	if m.selectionReady {
		sel[1] = fmt.Sprintf("%s, %d items", formatSizes(m.selection.Apparent, m.selection.Disk), m.selection.Count)
	}
	return append(res, sel)
}

// Move renderIndex by delta rows, wrapping at both ends.
// The last row can only be reached as the last visible line.
func (m *Model) moveRenderIndexBy(delta int) {
	if m.MetadataLen() == 0 {
		return
	}
	l := m.maxRenderIndex() + 1
	m.renderIndex = ((m.renderIndex+delta)%l + l) % l
}

func (m *Model) getPageScrollSize() int {
	scrollSize := common.Config.PageScrollSize
	if scrollSize <= 0 {
		// Use default full page behavior
		scrollSize = m.height - borderSize
	}
	// height can be tiny on small terminals, so keep moving at least one row
	if scrollSize < 1 {
		scrollSize = 1
	}
	return scrollSize
}

// Control metadata panel up
func (m *Model) ListUp() {
	m.moveRenderIndexBy(-1)
}

// Control metadata panel down
func (m *Model) ListDown() {
	m.moveRenderIndexBy(1)
}

// Control metadata panel page up
func (m *Model) PgUp() {
	m.moveRenderIndexBy(-m.getPageScrollSize())
}

// Control metadata panel page down
func (m *Model) PgDown() {
	m.moveRenderIndexBy(m.getPageScrollSize())
}

func (m *Model) SetBlank() {
	m.metadata.filepath = ""
	m.metadata.data = m.metadata.data[:0]
	m.metadata.infoMsg = "No metadata present"
}

func (m *Model) IsBlank() bool {
	return m.MetadataLen() == 0 && m.metadata.infoMsg == ""
}

func (m *Model) SetInfoMsg(msg string) {
	m.metadata.infoMsg = msg
}

func (m *Model) Render(metadataFocused bool) string {
	r := ui.MetadataRenderer(m.height, m.width, metadataFocused)
	if m.MetadataLen() == 0 {
		r.AddLines("", " "+m.metadata.infoMsg)
		return r.Render()
	}
	m.renderIndex = min(m.renderIndex, m.maxRenderIndex())
	data := m.displayData()
	keyLen, valueLen := computeRenderDimensions(data, m.width-2-keyValueSpacingLen)
	r.SetBorderInfoItems(fmt.Sprintf("%d/%d", m.renderIndex+1, len(data)))
	lines := formatMetadataLines(data, m.renderIndex, m.height-borderSize, keyLen, valueLen)
	m.highlightLines(lines, data, keyLen, valueLen, metadataFocused)
	r.AddLines(lines...)
	return r.Render()
}

// highlightLines adds styling to the visible lines: dimmed labels, the dimmed zone part of dates,
// and the selected permission bit when the metadata panel is focused.
func (m *Model) highlightLines(lines []string, data [][2]string, keyLen int, valueLen int, focused bool) {
	normal := lipgloss.NewStyle().Foreground(common.FooterFGColor).Background(common.FooterBGColor)
	dimmed := normal.Faint(true)
	if c := common.DimmedColor(common.FooterFGColor, common.FooterBGColor); c != nil {
		dimmed = normal.Foreground(c)
	}
	labelStyle := dimmed
	for i := m.renderIndex; i < min(m.renderIndex+len(lines), len(data)); i++ {
		key, value := data[i][0], data[i][1]
		label := labelStyle.Render(fmt.Sprintf("%-*s%s",
			keyLen, common.TruncateMiddleText(key, keyLen, "..."), keyValueSpacing))
		if ansi.StringWidth(value) > valueLen {
			lines[i-m.renderIndex] = label + normal.Render(common.TruncateMiddleText(value, valueLen, "..."))
			continue
		}
		var styledValue string
		switch {
		case focused && m.metadata.hasMode && key == keyPermissions:
			bit := m.selectedPermBit
			styledValue = normal.Render(value[:bit]) + common.FilePanelItemSelectedStyle.Render(value[bit:bit+1]) +
				normal.Render(value[bit+1:])
		case key == keyOwnerGroup && strings.HasPrefix(value, rootOwner+" / "):
			rootStyle := normal.Foreground(common.ProcessErrorStyle.GetForeground()).Bold(true)
			styledValue = rootStyle.Render(rootOwner) + normal.Render(value[len(rootOwner):])
		case (key == keyDataModified || key == keyDataAccessed) && len(value) >= dateHighlightLen:
			styledValue = normal.Render(value[:dateHighlightLen]) + dimmed.Render(value[dateHighlightLen:])
		default:
			styledValue = normal.Render(value)
		}
		lines[i-m.renderIndex] = label + styledValue
	}
}
