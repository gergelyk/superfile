package metadata

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/yorukot/superfile/src/internal/common"
)

// Usage is the cumulative size of a set of files and directories.
type Usage struct {
	Count    int   // number of top level items
	Apparent int64 // sum of file sizes. Directory inodes are not counted
	Disk     int64 // space allocated on disk, including directory inodes
}

// DiskUsage walks all paths recursively (without following symlinks) and sums their sizes.
func DiskUsage(paths []string) Usage {
	res := Usage{Count: len(paths)}
	for _, path := range paths {
		walkErr := filepath.WalkDir(path, func(_ string, entry fs.DirEntry, err error) error {
			if err != nil {
				slog.Debug("Disk usage walk error", "error", err)
				// Skip unreadable entries and keep going
				return nil
			}
			info, infoErr := entry.Info()
			if infoErr != nil {
				return nil //nolint:nilerr // best effort size calculation
			}
			if !entry.IsDir() {
				res.Apparent += info.Size()
			}
			res.Disk += getSizeOnDisk(info)
			return nil
		})
		if walkErr != nil {
			slog.Error("errors during WalkDir", "error", walkErr)
		}
	}
	return res
}

// getFileType returns the description reported by the `file` command, without the file name.
func getFileType(path string) (string, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), fileCommandTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "file", "-b", "--", path).Output()
	if err != nil {
		return "", false
	}
	res := strings.TrimSpace(string(out))
	return res, res != ""
}

// formatSizes formats the actual size together with the space occupied on disk, like "1.0 KiB / 4.0 KiB (on disk)".
func formatSizes(apparent int64, disk int64) string {
	return common.FormatFileSize(apparent) + " / " + common.FormatFileSize(disk) + " (on disk)"
}

// formatPermissions formats permission bits as symbolic and numeric form, like "rwx------ / 700".
// Setuid, setgid and sticky bits are only visible in the numeric form, as 4 digits.
func formatPermissions(mode os.FileMode) string {
	const (
		setuid  = 0o4000
		setgid  = 0o2000
		sticky  = 0o1000
		maxPerm = 0o777
	)
	bits := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		bits |= setuid
	}
	if mode&os.ModeSetgid != 0 {
		bits |= setgid
	}
	if mode&os.ModeSticky != 0 {
		bits |= sticky
	}
	numeric := fmt.Sprintf("%03o", bits)
	if bits > maxPerm {
		numeric = fmt.Sprintf("%04o", bits)
	}
	return symbolicPermissions(mode) + " / " + numeric
}

// symbolicPermissions returns the 9 characters like "rwxr-xr--"
func symbolicPermissions(mode os.FileMode) string {
	return mode.Perm().String()[1:]
}

// splitFileType splits a description from the `file` command into at most two lines.
// It prefers to split after a comma, picking the one that makes lines the most even.
// A text without commas is split at a space only if it is longer than maxWidth.
func splitFileType(text string, maxWidth int) []string {
	splitAt := -1
	best := len(text)
	for i, ch := range text {
		if ch != ',' || i+1 >= len(text) {
			continue
		}
		if diff := abs(len(text[:i+1]) - len(strings.TrimSpace(text[i+1:]))); diff < best {
			best, splitAt = diff, i+1
		}
	}
	if splitAt < 0 && len(text) > maxWidth {
		// Space closest to the middle
		for i, ch := range text {
			if ch == ' ' && (splitAt < 0 || abs(i-len(text)/2) < abs(splitAt-len(text)/2)) {
				splitAt = i
			}
		}
	}
	if splitAt <= 0 {
		return []string{text}
	}
	return []string{strings.TrimSpace(text[:splitAt]), strings.TrimSpace(text[splitAt:])}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
