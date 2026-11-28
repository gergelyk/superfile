package internal

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/yorukot/superfile/src/internal/ui/notify"
)

const (
	symlinkRelativeKey = "r"
	symlinkAbsoluteKey = "a"
)

// startSymlink asks how to link when the clipboard has exactly one directory
func (m *model) startSymlink() {
	items := m.clipboard.PruneInaccessibleItemsAndGet()
	if len(items) != 1 {
		m.showNotify("Cannot create symlink", "The clipboard must contain exactly one directory.", notify.NoAction)
		return
	}
	info, err := os.Stat(items[0])
	if err != nil || !info.IsDir() {
		m.showNotify("Cannot create symlink", "The clipboard must contain exactly one directory.", notify.NoAction)
		return
	}
	m.pendingSymlink = items[0]
	m.showNotify("Create symlink to "+filepath.Base(items[0]),
		"In: "+m.getFocusedFilePanel().Location+"\nShould the link use a relative or an absolute path?",
		notify.SymlinkAction)
}

// symlinkChoiceKey handles keys of the dialog that asks for the path kind
func (m *model) symlinkChoiceKey(msg string) tea.Cmd {
	switch {
	case msg == symlinkRelativeKey:
		m.notifyModel.Close()
		m.createPendingSymlink(true)
	case msg == symlinkAbsoluteKey:
		m.notifyModel.Close()
		m.createPendingSymlink(false)
	case isCancelKey(msg):
		m.notifyModel.Close()
		m.pendingSymlink = ""
	}
	return nil
}

func (m *model) createPendingSymlink(relative bool) {
	source := m.pendingSymlink
	m.pendingSymlink = ""
	dir := m.getFocusedFilePanel().Location
	if err := createSymlink(source, dir, relative); err != nil {
		m.showNotify("Cannot create symlink", err.Error(), notify.NoAction)
		return
	}
	m.fileModel.UpdateFilePanelsIfNeeded(true)
}

// createSymlink creates a link to the directory source inside dir, and does not overwrite anything
func createSymlink(source string, dir string, relative bool) error {
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	target := source
	if relative {
		// A relative target is resolved from the real location of the link
		realDir, dirErr := filepath.EvalSymlinks(dir)
		realSource, srcErr := filepath.EvalSymlinks(source)
		if err = errors.Join(dirErr, srcErr); err != nil {
			return err
		}
		if target, err = filepath.Rel(realDir, realSource); err != nil {
			return err
		}
	}
	name := filepath.Base(source)
	link := filepath.Join(dir, name)
	for i := 1; ; i++ {
		if _, statErr := os.Lstat(link); os.IsNotExist(statErr) {
			break
		}
		link = filepath.Join(dir, fmt.Sprintf("%s-%d", name, i))
	}
	return os.Symlink(target, link)
}
