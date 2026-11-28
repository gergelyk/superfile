package common

import (
	"os"
	"runtime"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestFileNameColor(t *testing.T) {
	oldTheme := Theme
	t.Cleanup(func() { Theme = oldTheme })
	Theme = ThemeType{FilePanelFG: "#111111", FilePanelDirFG: "#222222"}

	assert.Equal(t, lipgloss.Color("#222222"), FileNameColor(os.ModeDir|0o755), "theme value is used")
	assert.Equal(t, lipgloss.Color(defaultSymlinkFG), FileNameColor(os.ModeSymlink|os.ModeDir),
		"symlink wins over dir, empty theme value falls back to default")
	assert.Equal(t, lipgloss.Color(defaultSpecialFG), FileNameColor(os.ModeDevice|os.ModeCharDevice))
	assert.Equal(t, lipgloss.Color(defaultSpecialFG), FileNameColor(os.ModeNamedPipe))
	assert.Equal(t, lipgloss.Color(defaultSpecialFG), FileNameColor(os.ModeSocket))
	assert.Equal(t, lipgloss.Color("#111111"), FileNameColor(0o644), "regular files use file_panel_fg")
	if runtime.GOOS != "windows" {
		assert.Equal(t, lipgloss.Color(defaultExecutableFG), FileNameColor(0o755))
	}
}

func TestRenderFileNameDimsExtension(t *testing.T) {
	style := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffffff"))
	bg := lipgloss.Color("#000000")

	out := RenderFileName("main.go", 10, style, bg, 0o644, 0, 0)
	assert.Equal(t, "main.go   ", ansi.Strip(out), "padded to the requested width")
	assert.NotEqual(t, style.Background(bg).Render("main.go")+"   ", out, "extension is styled differently")
	assert.Equal(t, 10, ansi.StringWidth(RenderFileName("a-very-long-name.txt", 10, style, bg, 0o644, 0, 0)))

	plain := style.Background(bg).Width(10)
	assert.Equal(t, plain.Render(".gitignore"[:10]), RenderFileName(".gitignore", 10, style, bg, 0o644, 0, 0))
	assert.Equal(t, plain.Render("dir.d"), RenderFileName("dir.d", 10, style, bg, os.ModeDir, 0, 0))
}

func TestRenderFileNameSuffixModes(t *testing.T) {
	t.Cleanup(func() { SuffixMode = SuffixInline })
	style := lipgloss.NewStyle()
	bg := lipgloss.Color("#000000")
	render := func(name string, mode os.FileMode) string {
		return ansi.Strip(RenderFileName(name, 10, style, bg, mode, 2, 4))
	}

	assert.Equal(t, "main.go   ", render("main.go", 0o644))

	CycleSuffixMode()
	assert.Equal(t, SuffixRight, SuffixMode)
	assert.Equal(t, "main go   ", render("main.go", 0o644))
	assert.Equal(t, "dir       ", render("dir", os.ModeDir))

	CycleSuffixMode()
	assert.Equal(t, SuffixHidden, SuffixMode)
	assert.Equal(t, "main      ", render("main.go", 0o644))
	assert.Zero(t, SuffixWidth("main.go", 0o644))

	CycleSuffixMode()
	assert.Equal(t, SuffixLeft, SuffixMode)
	assert.Equal(t, "main      ", render("main.go", 0o644))
	assert.Equal(t, "go ", ansi.Strip(RenderSuffixColumn("main.go", style, bg, 0o644, 2)))
	assert.Equal(t, "   ", ansi.Strip(RenderSuffixColumn("dir", style, bg, os.ModeDir, 2)))
	assert.Equal(t, 2, SuffixWidth("main.go", 0o644))

	CycleSuffixMode()
	assert.Equal(t, SuffixInline, SuffixMode)
}

func TestRenderSymlinkName(t *testing.T) {
	style := lipgloss.NewStyle()
	bg := lipgloss.Color("#000000")
	out := func(width int) string {
		return ansi.Strip(renderSymlinkName("link", "../dir", width, style, bg, os.ModeSymlink))
	}
	assert.Equal(t, "link -> ../dir  ", out(16))
	assert.Equal(t, "link -> ...", out(11))
}
