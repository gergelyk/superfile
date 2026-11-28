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

// startSymlink asks how to link the items in the clipboard
func (m *model) startSymlink() {
	items := m.clipboard.PruneInaccessibleItemsAndGet()
	if len(items) == 0 {
		m.showNotify("Cannot create symlink", "The clipboard is empty.", notify.NoAction)
		return
	}
	m.pendingSymlinks = items
	title := "Create symlink to " + filepath.Base(items[0])
	if len(items) > 1 {
		title = fmt.Sprintf("Create %d symlinks", len(items))
	}
	m.showNotify(title,
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
		m.pendingSymlinks = nil
	}
	return nil
}

func (m *model) createPendingSymlink(relative bool) {
	sources := m.pendingSymlinks
	m.pendingSymlinks = nil
	dir := m.getFocusedFilePanel().Location
	var errs []error
	for _, source := range sources {
		if err := createSymlink(source, dir, relative); err != nil {
			errs = append(errs, err)
		}
	}
	m.fileModel.UpdateFilePanelsIfNeeded(true)
	if err := errors.Join(errs...); err != nil {
		m.showNotify("Cannot create symlink", err.Error(), notify.NoAction)
	}
}

// createSymlink creates a link to source inside dir, and does not overwrite anything
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
