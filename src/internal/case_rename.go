package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/yorukot/superfile/src/internal/common"
)

// caseRenamedName converts the name of an item to the given case, keeping the extension.
func caseRenamedName(name string, isDir bool, c common.NameCase) string {
	stemEnd := renameSelectionEnd(name, isDir)
	runes := []rune(name)
	stem, ext := string(runes[:stemEnd]), string(runes[stemEnd:])
	if c == common.UpperCase {
		ext = strings.ToUpper(ext)
	}
	return common.ConvertCase(stem, c) + ext
}

// caseRenamedNames returns the new names for all items. Numbered names depend on the item position.
func caseRenamedNames(paths []string, c common.NameCase) []string {
	names := make([]string, len(paths))
	digits := len(strconv.Itoa(len(paths)))
	for i, p := range paths {
		name := filepath.Base(p)
		isDir := isDirectory(p)
		if c != common.NumberedCase {
			names[i] = caseRenamedName(name, isDir, c)
			continue
		}
		ext := name[renameSelectionEnd(name, isDir):]
		names[i] = fmt.Sprintf("%0*d", digits, i+1) + ext
	}
	return names
}

func isDirectory(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
