package archive

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// Modifications stream the archive into a temporary file next to it, which
// then atomically replaces the original. Nothing is unpacked to disk.

// sink receives new entries that are added to the rewritten archive
type sink interface {
	add(name string, info fs.FileInfo, link string, r io.Reader) error
}

type mapFunc func(name string) (string, bool)

// Delete removes the given paths (files or directories) from their archive.
// All paths must belong to the same archive.
func Delete(paths []string) error {
	arch, inners, err := splitAll(paths)
	if err != nil {
		return err
	}
	return rewrite(arch, func(name string) (string, bool) {
		for _, in := range inners {
			if name == in || strings.HasPrefix(name, in+"/") {
				return "", false
			}
		}
		return name, true
	}, nil)
}

// Rename renames a file or directory inside an archive
func Rename(oldPath, newPath string) error {
	arch, oldInner, ok := Split(oldPath)
	arch2, newInner, ok2 := Split(newPath)
	if !ok || !ok2 || arch != arch2 || oldInner == "" || newInner == "" {
		return errors.New("rename must stay inside one archive")
	}
	idx, err := load(arch)
	if err != nil {
		return err
	}
	if _, exists := idx.nodes[newInner]; exists {
		return fmt.Errorf("%s already exists", newInner)
	}
	if strings.HasPrefix(newInner, oldInner+"/") {
		return errors.New("cannot rename an item into itself")
	}
	return rewrite(arch, func(name string) (string, bool) {
		if name == oldInner || strings.HasPrefix(name, oldInner+"/") {
			return newInner + strings.TrimPrefix(name, oldInner), true
		}
		return name, true
	}, nil)
}

// Add copies real files or directories into the directory dstDir of an
// archive. Names clashing with existing entries get a "(N)" suffix.
func Add(dstDir string, sources []string) error {
	arch, inner, ok := Split(dstDir)
	if !ok {
		return fmt.Errorf("%s is not inside an archive", dstDir)
	}
	idx, err := load(arch)
	if err != nil {
		return err
	}
	if e, exists := idx.nodes[inner]; !exists || !e.dir {
		return fmt.Errorf("%s is not a directory", dstDir)
	}
	names := make([]string, len(sources))
	for i, src := range sources {
		if names[i], err = UniqueName(dstDir, filepath.Base(src)); err != nil {
			return err
		}
	}
	return rewrite(arch, func(name string) (string, bool) { return name, true }, func(s sink) error {
		for i, src := range sources {
			top := path.Join(inner, names[i])
			err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(src, p)
				if err != nil {
					return err
				}
				name := top
				if rel != "." {
					name = path.Join(top, filepath.ToSlash(rel))
				}
				return addReal(s, name, p, info)
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func addReal(s sink, name, p string, info fs.FileInfo) error {
	switch {
	case info.IsDir():
		return s.add(name, info, "", nil)
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(p)
		if err != nil {
			return err
		}
		return s.add(name, info, target, nil)
	case info.Mode().IsRegular():
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		return s.add(name, info, "", f)
	}
	return nil
}

func splitAll(paths []string) (string, []string, error) {
	var arch string
	inners := make([]string, 0, len(paths))
	for _, p := range paths {
		a, inner, ok := Split(p)
		if !ok || inner == "" {
			return "", nil, fmt.Errorf("%s is not inside an archive", p)
		}
		if arch != "" && a != arch {
			return "", nil, errors.New("items belong to different archives")
		}
		arch = a
		inners = append(inners, inner)
	}
	if arch == "" {
		return "", nil, errors.New("nothing to do")
	}
	return arch, inners, nil
}

func rewrite(arch string, mapName mapFunc, adds func(sink) error) (err error) {
	st, err := os.Stat(arch)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(arch), ".spf-archive-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmp.Name())
		}
	}()

	if KindOf(arch) == KindZip {
		err = rewriteZip(arch, tmp, mapName, adds)
	} else {
		err = rewriteTar(arch, tmp, mapName, adds)
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err = os.Chmod(tmp.Name(), st.Mode().Perm()); err != nil {
		return err
	}
	defer invalidate(arch)
	return os.Rename(tmp.Name(), arch)
}

// zipSink

type zipSink struct{ zw *zip.Writer }

func (s zipSink) add(name string, info fs.FileInfo, link string, r io.Reader) error {
	h, err := zip.FileInfoHeader(info)
	if err != nil {
		return err
	}
	h.Name = name
	if info.IsDir() {
		h.Name += "/"
		_, err = s.zw.CreateHeader(h)
		return err
	}
	h.Method = zip.Deflate
	w, err := s.zw.CreateHeader(h)
	if err != nil {
		return err
	}
	if link != "" {
		_, err = io.WriteString(w, link)
		return err
	}
	_, err = io.Copy(w, r)
	return err
}

func rewriteZip(arch string, out io.Writer, mapName mapFunc, adds func(sink) error) error {
	zr, err := zip.OpenReader(arch)
	if err != nil {
		return err
	}
	defer zr.Close()
	zw := zip.NewWriter(out)
	for _, f := range zr.File {
		name, keep := mapName(cleanName(f.Name))
		if !keep {
			continue
		}
		h := f.FileHeader
		h.Name = name
		if f.FileInfo().IsDir() {
			h.Name += "/"
		}
		raw, err := f.OpenRaw()
		if err != nil {
			return err
		}
		w, err := zw.CreateRaw(&h)
		if err != nil {
			return err
		}
		if _, err = io.Copy(w, raw); err != nil {
			return err
		}
	}
	if adds != nil {
		if err = adds(zipSink{zw}); err != nil {
			return err
		}
	}
	return zw.Close()
}

// tarSink

type tarSink struct{ tw *tar.Writer }

func (s tarSink) add(name string, info fs.FileInfo, link string, r io.Reader) error {
	h, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	h.Name = name
	if info.IsDir() {
		h.Name += "/"
	}
	if err = s.tw.WriteHeader(h); err != nil {
		return err
	}
	if r != nil && h.Typeflag == tar.TypeReg {
		_, err = io.CopyN(s.tw, r, h.Size)
	}
	return err
}

func rewriteTar(arch string, out io.Writer, mapName mapFunc, adds func(sink) error) error {
	in, err := os.Open(arch)
	if err != nil {
		return err
	}
	defer in.Close()
	var src io.Reader = in
	var dst io.Writer = out
	var gw *gzip.Writer
	if KindOf(arch) == KindTgz {
		gz, gzErr := gzip.NewReader(in)
		if gzErr != nil {
			return gzErr
		}
		defer gz.Close()
		src = gz
		gw = gzip.NewWriter(out)
		dst = gw
	}
	tr := tar.NewReader(src)
	tw := tar.NewWriter(dst)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if h.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		name, keep := mapName(cleanName(h.Name))
		if !keep {
			continue
		}
		h.Name = name
		if h.FileInfo().IsDir() {
			h.Name += "/"
		}
		delete(h.PAXRecords, "path")
		h.Format = tar.FormatUnknown
		if err = tw.WriteHeader(h); err != nil {
			return err
		}
		if _, err = io.Copy(tw, tr); err != nil {
			return err
		}
	}
	if adds != nil {
		if err := adds(tarSink{tw}); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	if gw != nil {
		return gw.Close()
	}
	return nil
}

// Mkdir creates a directory (and missing parents) inside an archive
func Mkdir(p string) error {
	arch, inner, ok := Split(p)
	if !ok || inner == "" {
		return fmt.Errorf("%s is not inside an archive", p)
	}
	idx, err := load(arch)
	if err != nil {
		return err
	}
	if e, exists := idx.nodes[inner]; exists {
		if e.dir {
			return nil
		}
		return fmt.Errorf("%s already exists", inner)
	}
	// Parents that exist only implicitly are written out too
	var missing []string
	for q := inner; q != "."; q = path.Dir(q) {
		if _, exists := idx.nodes[q]; !exists {
			missing = append([]string{q}, missing...)
		}
	}
	now := time.Now()
	return rewrite(arch, func(name string) (string, bool) { return name, true }, func(s sink) error {
		for _, name := range missing {
			info := Info{entry{name: name, dir: true, mode: fs.ModeDir | dirPerm, mod: now}}
			if err := s.add(name, info, "", nil); err != nil {
				return err
			}
		}
		return nil
	})
}
