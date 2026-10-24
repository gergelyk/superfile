package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yorukot/superfile/src/internal/ui/filepanel"
	"github.com/yorukot/superfile/src/pkg/utils"
)

func TestPanelNumberLabel(t *testing.T) {
	assert.Equal(t, "1", filepanel.PanelNumberLabel(0))
	assert.Equal(t, "9", filepanel.PanelNumberLabel(8))
	assert.Equal(t, "0", filepanel.PanelNumberLabel(9))
	assert.Empty(t, filepanel.PanelNumberLabel(10))
}

func TestPanelTransferTarget(t *testing.T) {
	idx, cut := panelTransferTarget("3")
	assert.Equal(t, 2, idx)
	assert.False(t, cut)
	idx, cut = panelTransferTarget("0")
	assert.Equal(t, 9, idx)
	assert.False(t, cut)
	idx, cut = panelTransferTarget("@")
	assert.Equal(t, 1, idx)
	assert.True(t, cut)
	idx, cut = panelTransferTarget(")")
	assert.Equal(t, 9, idx)
	assert.True(t, cut)
	assert.False(t, isPanelTransferKey("a"))
	assert.False(t, isPanelTransferKey("12"))
}

func TestTransferToPanel(t *testing.T) {
	curTestDir := filepath.Join(testDir, "TestTransferToPanel")
	dir1 := filepath.Join(curTestDir, "dir1")
	dir2 := filepath.Join(curTestDir, "dir2")
	file1 := filepath.Join(dir1, "file1.txt")
	setup := func(t *testing.T) *TeaProg {
		t.Helper()
		utils.SetupDirectories(t, curTestDir, dir1, dir2)
		utils.SetupFiles(t, file1)
		t.Cleanup(func() { os.RemoveAll(curTestDir) })
		p := NewTestTeaProgWithEventLoop(t, defaultTestModel(dir1, dir2))
		require.Equal(t, 2, p.getModel().fileModel.PanelCount())
		return p
	}

	t.Run("Copy with number", func(t *testing.T) {
		p := setup(t)
		p.SendKey("2")
		assert.Eventually(t, func() bool {
			_, err := os.Lstat(filepath.Join(dir2, "file1.txt"))
			return err == nil
		}, DefaultTestTimeout, DefaultTestTick)
		assert.FileExists(t, file1)
	})

	t.Run("Move with shift+number", func(t *testing.T) {
		p := setup(t)
		p.SendKey("@")
		assert.Eventually(t, func() bool {
			_, err := os.Lstat(filepath.Join(dir2, "file1.txt"))
			return err == nil
		}, DefaultTestTimeout, DefaultTestTick)
		assert.NoFileExists(t, file1)
	})

	t.Run("Nonexistent panel is ignored", func(t *testing.T) {
		p := setup(t)
		assert.Nil(t, p.getModel().getTransferToPanelCmd(5, false))
		assert.Nil(t, p.getModel().getTransferToPanelCmd(0, false))
	})

	t.Run("Panels in same directory are ignored", func(t *testing.T) {
		utils.SetupDirectories(t, curTestDir, dir1)
		utils.SetupFiles(t, file1)
		t.Cleanup(func() { os.RemoveAll(curTestDir) })
		m := defaultTestModel(dir1, dir1)
		assert.Nil(t, m.getTransferToPanelCmd(1, false))
		assert.Nil(t, m.getTransferToPanelCmd(1, true))
	})

	t.Run("Number is on the top right border", func(t *testing.T) {
		utils.SetupDirectories(t, curTestDir, dir1, dir2)
		t.Cleanup(func() { os.RemoveAll(curTestDir) })
		m := defaultTestModel(dir1, dir2)
		panel := &m.fileModel.FilePanels[1]
		top := strings.Split(ansi.Strip(panel.Render(false, 1)), "\n")[0]
		assert.Contains(t, top, " 2 ")
		before, _, _ := strings.Cut(top, " 2 ")
		assert.Greater(t, ansi.StringWidth(before), ansi.StringWidth(top)/2)
	})
}

func TestTransferDoesNotOverwrite(t *testing.T) {
	curTestDir := filepath.Join(testDir, "TestTransferNoOverwrite")
	dir1 := filepath.Join(curTestDir, "dir1")
	dir2 := filepath.Join(curTestDir, "dir2")
	file1 := filepath.Join(dir1, "file1.txt")
	existing := filepath.Join(dir2, "file1.txt")
	for _, key := range []string{"2", "@"} {
		utils.SetupDirectories(t, curTestDir, dir1, dir2)
		utils.SetupFilesWithData(t, []byte("new"), file1)
		utils.SetupFilesWithData(t, []byte("old"), existing)
		p := NewTestTeaProgWithEventLoop(t, defaultTestModel(dir1, dir2))
		p.SendKey(key)
		renamed := filepath.Join(dir2, "file1(1).txt")
		assert.Eventually(t, func() bool {
			_, err := os.Lstat(renamed)
			return err == nil
		}, DefaultTestTimeout, DefaultTestTick)
		data, err := os.ReadFile(existing)
		require.NoError(t, err)
		assert.Equal(t, "old", string(data))
		os.RemoveAll(curTestDir)
	}
}
