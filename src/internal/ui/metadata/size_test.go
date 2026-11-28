package metadata

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiskUsage(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "sub")
	require.NoError(t, os.Mkdir(sub, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a"), make([]byte, 100), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(sub, "b"), make([]byte, 50), 0o644))
	single := filepath.Join(t.TempDir(), "c")
	require.NoError(t, os.WriteFile(single, make([]byte, 10), 0o644))

	usage := DiskUsage([]string{dir, single})
	assert.Equal(t, 2, usage.Count)
	assert.Equal(t, int64(160), usage.Apparent)
	assert.GreaterOrEqual(t, usage.Disk, int64(0))
}

func TestFormatPermissions(t *testing.T) {
	assert.Equal(t, "rwxr-xr-x / 755", formatPermissions(0o755))
	assert.Equal(t, "rwx------ / 700", formatPermissions(0o700|os.ModeDir))
	assert.Equal(t, "rwxr-xr-x / 4755", formatPermissions(0o755|os.ModeSetuid))
}

func TestTogglePermissionAndSelectionRow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "f")
	require.NoError(t, os.WriteFile(path, nil, 0o600))
	info, err := os.Lstat(path)
	require.NoError(t, err)

	m := New()
	data := [][2]string{
		{keyName, "f"}, {keyFileType, "ELF 64-bit, x86-64, dynamic"}, {keyPermissions, ""}, {keySize, "1B"},
	}
	meta := NewMetadata(data, path, "")
	meta.mode, meta.hasMode = info.Mode(), true
	m.SetMetadata(meta, false)
	rows0 := m.displayData()
	wantType := [][2]string{{keyFileType, "ELF 64-bit,"}, {"", "x86-64, dynamic"}}
	assert.Equal(t, wantType, rows0[1:3], "file type is split into two lines")
	assert.Equal(t, "rw------- / 600", m.displayData()[3][1])

	// Select group's r (bit 3) and toggle it
	m.MovePermissionSelection(3)
	_, err = m.TogglePermission()
	require.NoError(t, err)
	assert.Equal(t, "rw-r----- / 640", m.displayData()[3][1])
	info, err = os.Lstat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())

	// Selection does not leave the 0-8 range
	m.MovePermissionSelection(100)
	assert.Equal(t, permBitCount-1, m.selectedPermBit)
	m.MovePermissionSelection(-100)
	assert.Equal(t, 0, m.selectedPermBit)

	m.ExpectSelection("k")
	rows := m.displayData()
	assert.Equal(t, keySize, rows[4][0], "size is second last")
	assert.Equal(t, keySelectedSize, rows[5][0], "selection row goes last")
	assert.Equal(t, selectionCalculatingMsg, rows[5][1])

	m.SetSelectionUsage("stale", Usage{Count: 9})
	assert.Equal(t, selectionCalculatingMsg, m.displayData()[5][1], "stale results are ignored")
	m.SetSelectionUsage("k", Usage{Count: 2, Apparent: 5, Disk: 4096})
	assert.Contains(t, m.displayData()[5][1], "(on disk), 2 items")
}

func TestSplitFileType(t *testing.T) {
	assert.Equal(t, []string{"ASCII text"}, splitFileType("ASCII text", 40))
	assert.Equal(t, []string{"ELF 64-bit LSB pie executable,", "x86-64, version 1 (SYSV)"},
		splitFileType("ELF 64-bit LSB pie executable, x86-64, version 1 (SYSV)", 40))
	assert.Equal(t, []string{"UTF-8", "Unicode text"}, splitFileType("UTF-8 Unicode text", 10))
	assert.Equal(t, []string{"verylongwordwithoutspaces"}, splitFileType("verylongwordwithoutspaces", 10))
}
