package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanBulkRename(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{"a", "b", "other"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), nil, 0o600))
	}
	paths := []string{filepath.Join(dir, "a"), filepath.Join(dir, "b")}

	t.Run("swap and rename", func(t *testing.T) {
		changes, problems := planBulkRename(dir, paths, []string{"b", "a"})
		assert.Empty(t, problems)
		assert.Len(t, changes, 2)
	})
	t.Run("unchanged", func(t *testing.T) {
		changes, problems := planBulkRename(dir, paths, []string{"a", "b"})
		assert.Empty(t, problems)
		assert.Empty(t, changes)
	})
	t.Run("fixable errors", func(t *testing.T) {
		_, problems := planBulkRename(dir, paths, []string{"x", "x"})
		assert.Len(t, problems, 2)
		_, problems = planBulkRename(dir, paths, []string{"other", ""})
		assert.Len(t, problems, 2)
		_, problems = planBulkRename(dir, paths, []string{"a/b", ".."})
		assert.Len(t, problems, 2)
	})
}

func TestExecuteBulkRenameSwap(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	require.NoError(t, os.WriteFile(a, []byte("A"), 0o600))
	require.NoError(t, os.WriteFile(b, []byte("B"), 0o600))
	failures := executeBulkRename(dir, []renameChange{{a, b}, {b, a}})
	assert.Empty(t, failures)
	data, err := os.ReadFile(a)
	require.NoError(t, err)
	assert.Equal(t, "B", string(data))
}

func TestSplitEditedLines(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, splitEditedLines("a\r\nb\n"))
	assert.Equal(t, []string{"a", ""}, splitEditedLines("a\n\n"))
}

func TestRenameSelectionEnd(t *testing.T) {
	tests := []struct {
		name  string
		isDir bool
		want  int
	}{
		{"a.tar.gz", false, 1},
		{".bashrc", false, 7},
		{"report v1.2 final.txt", false, 17},
		{"my file.txt", false, 7},
		{"a. b", false, 4},
		{"dir.d", true, 5},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, renameSelectionEnd(tt.name, tt.isDir), tt.name)
	}
}
