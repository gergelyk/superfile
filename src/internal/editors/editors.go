// Package editors resolves which external editors can open a given item,
// based on the configured per file type editor lists.
package editors

import (
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/yorukot/superfile/src/internal/common"
	"github.com/yorukot/superfile/src/pkg/utils"
)

const (
	// SystemDefault is the entry that opens an item with the system default application
	SystemDefault = "default"
	// Terminal is the entry that opens an item with the terminal editor (editor config or $EDITOR)
	Terminal = "terminal"
)

type Editor struct {
	// Name is what the user sees in the picker
	Name string
	// Command is the command line (without the path), empty for special entries
	Command  string
	Terminal bool
}

// Resolve returns the editors that can open path. The system default is always
// included, as the last choice. Editors that are not installed are skipped.
func Resolve(path string, isDir bool) []Editor {
	var names []string
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	switch {
	case isDir:
		names = common.Config.Editors
	default:
		names = typeEditors(ext)
		if names == nil {
			if text, err := common.IsTextFile(path); err == nil && text {
				names = common.Config.Editors
			}
		}
	}

	var res []Editor
	seen := map[string]bool{}
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		switch name {
		case SystemDefault:
			res = append(res, Editor{Name: "System default"})
		case Terminal:
			res = append(res, Editor{Name: "Terminal editor", Terminal: true})
		default:
			parts := strings.Fields(name)
			if _, err := exec.LookPath(parts[0]); err == nil {
				res = append(res, Editor{Name: name, Command: name})
			}
		}
	}
	for _, n := range names {
		if n != SystemDefault {
			add(n)
		}
	}
	add(SystemDefault)
	return res
}

// typeEditors returns the editor list of the first file type that has ext, or nil
func typeEditors(ext string) []string {
	keys := make([]string, 0, len(common.Config.FileTypes))
	for k := range common.Config.FileTypes {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		ft := common.Config.FileTypes[k]
		for _, e := range ft.Extensions {
			if strings.EqualFold(strings.TrimPrefix(e, "."), ext) {
				return ft.Editors
			}
		}
	}
	return nil
}

// Launch starts a GUI editor (or the system default application) detached from the terminal
func (e Editor) Launch(path string) error {
	if e.Terminal {
		return errors.New("terminal editor cannot be launched detached")
	}
	var cmd *exec.Cmd
	if e.Command == "" {
		cmd = systemOpenCommand(path)
	} else {
		parts := strings.Fields(e.Command)
		cmd = exec.Command(parts[0], append(parts[1:], path)...) //nolint:gosec // user-configured editor
	}
	utils.DetachFromTerminal(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func systemOpenCommand(path string) *exec.Cmd {
	switch runtime.GOOS {
	case utils.OsDarwin:
		return exec.Command("open", path)
	case utils.OsWindows:
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		return exec.Command("xdg-open", path)
	}
}
