package editors

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yorukot/superfile/src/internal/common"
)

func names(list []Editor) []string {
	res := make([]string, len(list))
	for i, e := range list {
		res[i] = e.Name
	}
	return res
}

func TestResolve(t *testing.T) {
	old := common.Config
	t.Cleanup(func() { common.Config = old })
	common.Config.Editors = []string{"sh", "no-such-editor"}
	common.Config.FileTypes = map[string]common.FileTypeConfig{
		"code": {Extensions: []string{"go"}, Editors: []string{"cat", "sh"}},
	}
	dir := t.TempDir()
	write := func(name string, data []byte) string {
		p := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(p, data, 0o600))
		return p
	}

	assert.Equal(t, []string{"cat", "sh", "System default"}, names(Resolve(write("a.GO", []byte("x")), false)))
	assert.Equal(t, []string{"sh", "System default"}, names(Resolve(write("a.txt", []byte("x")), false)))
	assert.Equal(t, []string{"System default"}, names(Resolve(write("a.bin", []byte{0, 1, 2}), false)))
}
