// Package archive lets zip, tar and tar.gz files be browsed and modified like
// directories. A path inside an archive is the archive's own path followed by
// the entry path, e.g. /home/me/a.zip/docs/readme.txt.
package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha1" //nolint:gosec // only used to name temp directories
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Kind int

const (
	KindNone Kind = iota
	KindZip
	KindTar
	KindTgz
)

const (
	dirPerm  = 0o755
	filePerm = 0o644
	tmpPerm  = 0o700
)

// KindOf returns the archive kind according to the file name
func KindOf(name string) Kind {
	lower := strings.ToLower(name)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return KindZip
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		return KindTgz
	case strings.HasSuffix(lower, ".tar"):
		return KindTar
	}
	return KindNone
}

// IsArchiveName tells whether the file name looks like a supported archive
func IsArchiveName(name string) bool {
	return KindOf(name) != KindNone
}

// Split splits p into the archive file and the cleaned slash separated path
// within it ("" for the archive root).
func Split(p string) (string, string, bool) {
	p = filepath.Clean(p)
	for q := p; ; {
		if IsArchiveName(filepath.Base(q)) {
			if info, err := os.Stat(q); err == nil && info.Mode().IsRegular() {
				rel, err := filepath.Rel(q, p)
				if err != nil {
					return "", "", false
				}
				inner := filepath.ToSlash(rel)
				if inner == "." {
					inner = ""
				}
				return q, inner, true
			}
		}
		parent := filepath.Dir(q)
		if parent == q {
			return "", "", false
		}
		q = parent
	}
}

// IsLocation tells whether p is an archive file or something inside it
func IsLocation(p string) bool {
	_, _, ok := Split(p)
	return ok
}

// Contains tells whether p is something inside an archive (the archive file itself excluded)
func Contains(p string) bool {
	_, inner, ok := Split(p)
	return ok && inner != ""
}

type entry struct {
	name string
	dir  bool
	mode fs.FileMode
	size int64
	mod  time.Time
}

// Info implements fs.FileInfo
type Info struct{ e entry }

func (i Info) Name() string       { return path.Base(i.e.name) }
func (i Info) Size() int64        { return i.e.size }
func (i Info) Mode() fs.FileMode  { return i.e.mode }
func (i Info) ModTime() time.Time { return i.e.mod }
func (i Info) IsDir() bool        { return i.e.dir }
func (i Info) Sys() any           { return nil }

type index struct {
	mod      time.Time
	size     int64
	nodes    map[string]entry
	children map[string][]string
}

var (
	cacheMu sync.Mutex
	cache   = map[string]*index{}
)

func cleanName(name string) string {
	return strings.TrimPrefix(path.Clean("/"+name), "/")
}

func invalidate(arch string) {
	cacheMu.Lock()
	delete(cache, arch)
	cacheMu.Unlock()
}

func load(arch string) (*index, error) {
	st, err := os.Stat(arch)
	if err != nil {
		return nil, err
	}
	cacheMu.Lock()
	idx, ok := cache[arch]
	cacheMu.Unlock()
	if ok && idx.mod.Equal(st.ModTime()) && idx.size == st.Size() {
		return idx, nil
	}

	idx = &index{
		mod: st.ModTime(), size: st.Size(),
		nodes:    map[string]entry{"": {dir: true, mode: fs.ModeDir | dirPerm, mod: st.ModTime()}},
		children: map[string][]string{},
	}
	err = iterate(arch, func(e entry, _ func() (io.Reader, error)) error {
		if e.name == "" {
			return nil
		}
		for p := path.Dir(e.name); p != "."; p = path.Dir(p) {
			if _, exists := idx.nodes[p]; !exists {
				idx.nodes[p] = entry{name: p, dir: true, mode: fs.ModeDir | dirPerm, mod: e.mod}
			}
		}
		idx.nodes[e.name] = e
		return nil
	})
	if err != nil {
		return nil, err
	}
	for name := range idx.nodes {
		if name == "" {
			continue
		}
		parent := path.Dir(name)
		if parent == "." {
			parent = ""
		}
		idx.children[parent] = append(idx.children[parent], name)
	}
	cacheMu.Lock()
	cache[arch] = idx
	cacheMu.Unlock()
	return idx, nil
}

// Stat returns info about a path inside an archive
func Stat(p string) (fs.FileInfo, error) {
	arch, inner, ok := Split(p)
	if !ok {
		return nil, fs.ErrNotExist
	}
	idx, err := load(arch)
	if err != nil {
		return nil, err
	}
	e, ok := idx.nodes[inner]
	if !ok {
		return nil, fs.ErrNotExist
	}
	if inner == "" {
		e.name = filepath.Base(arch)
	}
	return Info{e}, nil
}

// Exists tells whether something exists at a path inside an archive
func Exists(p string) bool {
	_, err := Stat(p)
	return err == nil
}

// ReadDir lists a directory inside an archive, or a real directory
func ReadDir(p string) ([]os.DirEntry, error) {
	arch, inner, ok := Split(p)
	if !ok {
		return os.ReadDir(p)
	}
	idx, err := load(arch)
	if err != nil {
		return nil, err
	}
	e, ok := idx.nodes[inner]
	if !ok || !e.dir {
		return nil, fmt.Errorf("%s: %w", p, fs.ErrNotExist)
	}
	names := append([]string(nil), idx.children[inner]...)
	sort.Strings(names)
	res := make([]os.DirEntry, 0, len(names))
	for _, n := range names {
		res = append(res, fs.FileInfoToDirEntry(Info{idx.nodes[n]}))
	}
	return res, nil
}

// iterate calls fn for every entry. The reader is valid only during the call.
func iterate(arch string, fn func(e entry, open func() (io.Reader, error)) error) error {
	if KindOf(arch) == KindZip {
		return iterateZip(arch, fn)
	}
	f, err := os.Open(arch)
	if err != nil {
		return err
	}
	defer f.Close()
	var src io.Reader = f
	if KindOf(arch) == KindTgz {
		gz, gzErr := gzip.NewReader(f)
		if gzErr != nil {
			return gzErr
		}
		defer gz.Close()
		src = gz
	}
	tr := tar.NewReader(src)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		e := entry{
			name: cleanName(h.Name), dir: h.FileInfo().IsDir(), mode: h.FileInfo().Mode(),
			size: h.Size, mod: h.ModTime,
		}
		if err = fn(e, func() (io.Reader, error) { return tr, nil }); err != nil {
			return err
		}
	}
}

func iterateZip(arch string, fn func(e entry, open func() (io.Reader, error)) error) error {
	zr, err := zip.OpenReader(arch)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, f := range zr.File {
		e := entry{
			name: cleanName(f.Name), dir: f.FileInfo().IsDir(), mode: f.Mode(),
			size: int64(f.UncompressedSize64), mod: f.Modified,
		}
		var rc io.ReadCloser
		err = fn(e, func() (io.Reader, error) {
			var openErr error
			rc, openErr = f.Open()
			return rc, openErr
		})
		if rc != nil {
			rc.Close()
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// Extract copies the entry at src (inside an archive) to dst on the real
// filesystem. dst is the final path of the copied item.
func Extract(src, dst string) error {
	arch, inner, ok := Split(src)
	if !ok || inner == "" {
		return fmt.Errorf("%s is not inside an archive", src)
	}
	found := false
	err := iterate(arch, func(e entry, open func() (io.Reader, error)) error {
		if e.name != inner && !strings.HasPrefix(e.name, inner+"/") {
			return nil
		}
		found = true
		target := dst
		if rel := strings.TrimPrefix(strings.TrimPrefix(e.name, inner), "/"); rel != "" {
			target = filepath.Join(dst, filepath.FromSlash(rel))
		}
		switch {
		case e.dir:
			return os.MkdirAll(target, dirPerm)
		case !e.mode.IsRegular():
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(target), dirPerm); err != nil {
			return err
		}
		r, err := open()
		if err != nil {
			return err
		}
		perm := e.mode.Perm()
		if perm == 0 {
			perm = filePerm
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
		if err != nil {
			return err
		}
		_, err = io.Copy(out, r)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		return err
	})
	if err == nil && !found {
		return fmt.Errorf("%s: %w", src, fs.ErrNotExist)
	}
	return err
}

// Materialize makes a file available at a real path, extracting it to a
// temporary location if it is inside an archive.
func Materialize(p string) (string, error) {
	arch, inner, ok := Split(p)
	if !ok || inner == "" {
		return p, nil
	}
	st, err := os.Stat(arch)
	if err != nil {
		return "", err
	}
	key := fmt.Sprintf("%s|%d|%d|%s", arch, st.ModTime().UnixNano(), st.Size(), inner)
	sum := sha1.Sum([]byte(key)) //nolint:gosec // not security related
	dir := filepath.Join(os.TempDir(), "spf-archive-"+hex.EncodeToString(sum[:8]))
	target := filepath.Join(dir, path.Base(inner))
	if _, err = os.Stat(target); err == nil {
		return target, nil
	}
	if err = os.MkdirAll(dir, tmpPerm); err != nil {
		return "", err
	}
	if err = Extract(p, target); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	return target, nil
}

// UniqueName returns a name not used yet in the directory dir inside an archive
func UniqueName(dir, name string) (string, error) {
	arch, inner, ok := Split(dir)
	if !ok {
		return name, nil
	}
	idx, err := load(arch)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	candidate := name
	for i := 1; ; i++ {
		if _, exists := idx.nodes[path.Join(inner, candidate)]; !exists {
			return candidate, nil
		}
		candidate = fmt.Sprintf("%s-%d%s", base, i, ext)
	}
}
