package internal

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yorukot/superfile/src/internal/common"
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

func TestCaseRenamedNamesNumbered(t *testing.T) {
	paths := make([]string, 10)
	for i := range paths {
		paths[i] = "/nonexistent/file" + strconv.Itoa(i) + ".tar.gz"
	}
	names := caseRenamedNames(paths, common.NumberedCase)
	assert.Equal(t, "01.tar.gz", names[0])
	assert.Equal(t, "10.tar.gz", names[9])
}

func TestCaseRenamedNameSpaced(t *testing.T) {
	assert.Equal(t, "My File Name.xyz", caseRenamedName("my_file_name.xyz", false, common.NaturalCase))
	assert.Equal(t, "My_File_Name.xyz", caseRenamedName("myFileName.xyz", false, common.PascalSnakeCase))
}

func TestCaseRenamedName(t *testing.T) {
	assert.Equal(t, "my_file_name.tar.gz", caseRenamedName("MyFile-Name.tar.gz", false, common.SnakeCase))
	assert.Equal(t, "MyFileName.xyz", caseRenamedName("my_file_name.xyz", false, common.PascalCase))
	assert.Equal(t, "MY_FILE_NAME.XYZ", caseRenamedName("my file name.xyz", false, common.UpperCase))
	assert.Equal(t, "dashed-name.xyz", caseRenamedName("dashedName.xyz", false, common.KebabCase))
	assert.Equal(t, "my-dir-d", caseRenamedName("My.Dir.d", true, common.KebabCase))
	assert.Equal(t, "myfilename.xyz", caseRenamedName("MyFile_Name.xyz", false, common.FlatCase))
}

func TestRegexRenamedNames(t *testing.T) {
	paths := []string{"/x/IMG_001.jpg", "/x/IMG_002.jpg"}
	names, err := regexRenamedNames(paths, `IMG_(\d+)`, "photo-$1", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"photo-001.jpg", "photo-002.jpg"}, names)

	names, err = regexRenamedNames(paths, "", "x", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"IMG_001.jpg", "IMG_002.jpg"}, names)

	names, err = regexRenamedNames(paths, "jpg|001", "x", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"IMG_x.jpg", "IMG_002.jpg"}, names)

	names, err = regexRenamedNames(paths, "jpg|001", "x", true)
	require.NoError(t, err)
	assert.Equal(t, []string{"IMG_x.x", "IMG_002.x"}, names)

	names, err = regexRenamedNames(paths, "(.*)", "${1}", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"IMG_001.jpg", "IMG_002.jpg"}, names)

	_, err = regexRenamedNames(paths, "(", "x", false)
	require.Error(t, err)
}
