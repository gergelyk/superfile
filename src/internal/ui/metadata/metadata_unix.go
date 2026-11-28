//go:build !windows

package metadata

import (
	"os"
	"os/user"
	"strconv"
	"syscall"
)

func getOwnerAndGroup(fileInfo os.FileInfo) (string, string) {
	usr := ""
	grp := ""
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		uid := strconv.FormatUint(uint64(stat.Uid), 10)
		gid := strconv.FormatUint(uint64(stat.Gid), 10)
		if userData, err := user.LookupId(uid); err == nil {
			usr = userData.Username
		}
		if groupData, err := user.LookupGroupId(gid); err == nil {
			grp = groupData.Name
		}
	}
	return usr, grp
}

// getSizeOnDisk returns the space allocated for the file. st_blocks is always in 512 byte units.
func getSizeOnDisk(fileInfo os.FileInfo) int64 {
	const blockSize = 512
	if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok {
		return int64(stat.Blocks) * blockSize //nolint:unconvert // Blocks is not int64 on all platforms
	}
	return fileInfo.Size()
}
