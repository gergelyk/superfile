package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeFixture(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	f, err := os.Create(p)
	require.NoError(t, err)
	files := map[string]string{"a.txt": "hello", "dir/b.txt": "bee", "dir/sub/c.txt": "sea"}
	switch KindOf(name) {
	case KindZip:
		zw := zip.NewWriter(f)
		for n, c := range files {
			w, err := zw.Create(n)
			require.NoError(t, err)
			_, err = w.Write([]byte(c))
			require.NoError(t, err)
		}
		require.NoError(t, zw.Close())
	default:
		var tw *tar.Writer
		var gw *gzip.Writer
		if KindOf(name) == KindTgz {
			gw = gzip.NewWriter(f)
			tw = tar.NewWriter(gw)
		} else {
			tw = tar.NewWriter(f)
		}
		for n, c := range files {
			require.NoError(t, tw.WriteHeader(&tar.Header{Name: n, Mode: 0o644, Size: int64(len(c))}))
			_, err := tw.Write([]byte(c))
			require.NoError(t, err)
		}
		require.NoError(t, tw.Close())
		if gw != nil {
			require.NoError(t, gw.Close())
		}
	}
	require.NoError(t, f.Close())
	return p
}

func names(t *testing.T, p string) []string {
	t.Helper()
	es, err := ReadDir(p)
	require.NoError(t, err)
	res := []string{}
	for _, e := range es {
		res = append(res, e.Name())
	}
	return res
}

func TestArchiveBrowseAndModify(t *testing.T) {
	for _, name := range []string{"x.zip", "x.tar", "x.tar.gz", "x.tgz"} {
		t.Run(name, func(t *testing.T) {
			arch := writeFixture(t, name)
			require.True(t, IsLocation(arch))
			require.False(t, Contains(arch))
			require.Equal(t, []string{"a.txt", "dir"}, names(t, arch))
			require.Equal(t, []string{"b.txt", "sub"}, names(t, filepath.Join(arch, "dir")))

			out := filepath.Join(t.TempDir(), "out")
			require.NoError(t, Extract(filepath.Join(arch, "dir"), out))
			data, err := os.ReadFile(filepath.Join(out, "sub", "c.txt"))
			require.NoError(t, err)
			require.Equal(t, "sea", string(data))

			require.NoError(t, Rename(filepath.Join(arch, "dir"), filepath.Join(arch, "renamed")))
			require.Equal(t, []string{"a.txt", "renamed"}, names(t, arch))
			require.Equal(t, []string{"b.txt", "sub"}, names(t, filepath.Join(arch, "renamed")))

			require.NoError(t, Delete([]string{filepath.Join(arch, "a.txt")}))
			require.Equal(t, []string{"renamed"}, names(t, arch))

			src := filepath.Join(t.TempDir(), "new")
			require.NoError(t, os.MkdirAll(filepath.Join(src, "in"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(src, "in", "f.txt"), []byte("eff"), 0o644))
			require.NoError(t, Add(filepath.Join(arch, "renamed"), []string{src}))
			require.NoError(t, Add(filepath.Join(arch, "renamed"), []string{src}))
			require.Equal(t, []string{"b.txt", "new", "new-1", "sub"}, names(t, filepath.Join(arch, "renamed")))
			p, err := Materialize(filepath.Join(arch, "renamed", "new-1", "in", "f.txt"))
			require.NoError(t, err)
			data, err = os.ReadFile(p)
			require.NoError(t, err)
			require.Equal(t, "eff", string(data))
		})
	}
}

func TestMkdir(t *testing.T) {
	for _, name := range []string{"x.zip", "x.tar", "x.tgz"} {
		arch := writeFixture(t, name)
		require.NoError(t, Mkdir(filepath.Join(arch, "dir", "new", "deep")))
		require.Equal(t, []string{"b.txt", "new", "sub"}, names(t, filepath.Join(arch, "dir")))
		require.Equal(t, []string{"deep"}, names(t, filepath.Join(arch, "dir", "new")))
		require.Error(t, Mkdir(filepath.Join(arch, "a.txt")))
	}
}
