package common

import (
	"image/color"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

const (
	defaultDirFG        = "#5f87ff"
	defaultSymlinkFG    = "#ffaf00"
	defaultSpecialFG    = "#ff5f87"
	defaultExecutableFG = "#5fd75f"

	executableBits = 0o111
	specialModes   = os.ModeDevice | os.ModeCharDevice | os.ModeSocket | os.ModeNamedPipe | os.ModeIrregular
)

func colorOrDefault(configured string, fallback string) color.Color {
	if configured == "" {
		configured = fallback
	}
	return lipgloss.Color(configured)
}

// FileNameColor returns the foreground color for a file name, based on its type.
// mode is expected to come from lstat, so symlinks are not followed.
func FileNameColor(mode os.FileMode) color.Color {
	switch {
	case mode&os.ModeSymlink != 0:
		return colorOrDefault(Theme.FilePanelSymlinkFG, defaultSymlinkFG)
	case mode.IsDir():
		return colorOrDefault(Theme.FilePanelDirFG, defaultDirFG)
	case mode&specialModes != 0:
		return colorOrDefault(Theme.FilePanelSpecialFileFG, defaultSpecialFG)
	// Permission bits do not indicate executability on Windows
	case runtime.GOOS != "windows" && mode&executableBits != 0:
		return colorOrDefault(Theme.FilePanelExecutableFG, defaultExecutableFG)
	default:
		return colorOrDefault(Theme.FilePanelFileFG, Theme.FilePanelFG)
	}
}

// FileNameStyle returns the style for a file name in the file panel.
// Selected items keep the selection style so that selection stays visible.
func FileNameStyle(mode os.FileMode, isSelected bool) lipgloss.Style {
	if isSelected {
		return FilePanelItemSelectedStyle
	}
	return FilePanelStyle.Foreground(FileNameColor(mode))
}

// dimmedColor mixes fg with bg to make fg less prominent. If bg is not a real color, it returns nil.
func dimmedColor(fg color.Color, bg color.Color) color.Color {
	const dimPercent = 55 // share of fg that is kept
	return mixColors(fg, bg, dimPercent)
}

// CursorRowBackground returns the background for the row with the cursor, a bit different from
// the panel background. It returns nil if the colors are not known.
func CursorRowBackground() color.Color {
	const cursorRowPercent = 10 // share of the foreground that is mixed in
	return mixColors(FilePanelFGColor, FilePanelBGColor, cursorRowPercent)
}

// mixColors keeps fgPercent percent of fg and the rest comes from bg. If bg is not a real color, it returns nil.
func mixColors(fg color.Color, bg color.Color, dimPercent uint32) color.Color {
	const (
		percentMax   = 100
		colorMax     = 0xffff
		byteMax      = 0xff
		byteMaxAlpha = byteMax
	)
	if fg == nil || bg == nil {
		return nil
	}
	fr, fgr, fb, fa := fg.RGBA()
	br, bgr, bb, ba := bg.RGBA()
	if fa == 0 || ba == 0 {
		return nil
	}
	mix := func(f, b uint32) uint8 {
		// Values are alpha premultiplied 16 bit, and are scaled down to 8 bit
		mixed := (f*dimPercent + b*(percentMax-dimPercent)) / percentMax
		return uint8(mixed * byteMax / colorMax) //nolint:gosec // <= 0xff
	}
	return color.RGBA{R: mix(fr, br), G: mix(fgr, bgr), B: mix(fb, bb), A: byteMaxAlpha}
}

// SuffixPlacement defines how the file suffixes (extensions) are shown.
type SuffixPlacement int

const (
	// SuffixInline keeps the suffix at the end of the name
	SuffixInline SuffixPlacement = iota
	// SuffixLeft puts suffixes of all files in a column before the names, without the leading period
	SuffixLeft
	// SuffixRight puts suffixes of all files in a column after the names, without the leading period
	SuffixRight
	// SuffixHidden does not show suffixes
	SuffixHidden
	SuffixModeCount
)

//nolint:gochecknoglobals // view setting shared by all file panels
var SuffixMode SuffixPlacement

func SetSuffixMode(mode SuffixPlacement) {
	SuffixMode = mode
}

// suffixCycle is the order of the views: left column, at the end of the name, right column, hidden
//
//nolint:gochecknoglobals // constant order
var suffixCycle = [...]SuffixPlacement{SuffixLeft, SuffixInline, SuffixRight, SuffixHidden}

func CycleSuffixMode() {
	for i, mode := range suffixCycle {
		if mode == SuffixMode {
			SuffixMode = suffixCycle[(i+1)%len(suffixCycle)]
			return
		}
	}
	SuffixMode = SuffixInline
}

// SplitSuffix splits a name into the stem and the suffix, with the leading period.
// Directories and symlinks have no suffix, and names like ".gitignore" are not split.
func SplitSuffix(name string, mode os.FileMode) (string, string) {
	if mode.IsDir() || mode&os.ModeSymlink != 0 {
		return name, ""
	}
	ext := filepath.Ext(name)
	if len(ext) == len(name) {
		return name, ""
	}
	return name[:len(name)-len(ext)], ext
}

// SuffixColumnMinWidth is the minimal width of the column with suffixes
const SuffixColumnMinWidth = 4

// SuffixColumnWidth returns the width of the suffix column, given the widest suffix. It is 0 without a column.
func SuffixColumnWidth(widest int) int {
	if SuffixMode != SuffixLeft && SuffixMode != SuffixRight {
		return 0
	}
	return max(widest, SuffixColumnMinWidth)
}

// SuffixWidth returns the width of the suffix column entry of the file, or 0 if there is no such column
func SuffixWidth(name string, mode os.FileMode) int {
	if SuffixMode != SuffixLeft && SuffixMode != SuffixRight {
		return 0
	}
	_, ext := SplitSuffix(name, mode)
	return ansi.StringWidth(strings.TrimPrefix(ext, "."))
}

// DimmedColor mixes fg with bg to make fg less prominent. It returns nil if bg is not a real color.
func DimmedColor(fg color.Color, bg color.Color) color.Color {
	return dimmedColor(fg, bg)
}

func dimStyle(base lipgloss.Style, fg color.Color, bgColor color.Color) lipgloss.Style {
	if dimmed := dimmedColor(fg, bgColor); dimmed != nil {
		return base.Foreground(dimmed)
	}
	return base.Faint(true)
}

// RenderSuffixColumn renders the suffix of the name for the column before the icon,
// followed by a space. It is empty unless suffixes are in the left column.
func RenderSuffixColumn(
	name string, style lipgloss.Style, bgColor color.Color, mode os.FileMode, extWidth int,
) string {
	if SuffixMode != SuffixLeft || extWidth == 0 {
		return ""
	}
	_, ext := SplitSuffix(name, mode)
	base := style.Background(bgColor)
	return dimStyle(base, style.GetForeground(), bgColor).Width(extWidth).Render(strings.TrimPrefix(ext, ".")) +
		base.Render(" ")
}

// RenderFileName renders a file name padded to width, with the suffix dimmed.
// extWidth is the widest suffix and stemWidth the widest name without suffix among the shown files.
// They are used when suffixes are placed in a column.
func RenderFileName(
	name string, width int, style lipgloss.Style, bgColor color.Color, mode os.FileMode, extWidth int, stemWidth int,
) string {
	stem, ext := SplitSuffix(name, mode)
	base := style.Background(bgColor)
	if SuffixMode == SuffixHidden {
		return base.Width(width).Render(ansi.Truncate(stem, width, "..."))
	}
	dim := dimStyle(base, style.GetForeground(), bgColor)

	const minStemWidth = 4
	if SuffixMode == SuffixLeft {
		return base.Width(width).Render(ansi.Truncate(stem, width, "..."))
	}
	if SuffixMode == SuffixRight && extWidth > 0 && width >= extWidth+minStemWidth {
		// The suffixes follow the longest name, they are not pushed to the right edge
		stemColumn := min(stemWidth+1, width-extWidth)
		return base.Width(stemColumn).Render(ansi.Truncate(stem, stemColumn-1, "...")) +
			dim.Width(extWidth).Render(strings.TrimPrefix(ext, ".")) +
			base.Width(max(width-stemColumn-extWidth, 0)).Render("")
	}

	truncated := ansi.Truncate(name, width, "...")
	extStart := len(stem)
	if ext == "" || extStart == 0 || len(truncated) <= extStart || !strings.HasPrefix(name, truncated[:extStart]) {
		return base.Width(width).Render(truncated)
	}
	padding := width - ansi.StringWidth(truncated)
	return base.Render(truncated[:extStart]) + dim.Render(truncated[extStart:]) +
		base.Render(strings.Repeat(" ", max(padding, 0)))
}
