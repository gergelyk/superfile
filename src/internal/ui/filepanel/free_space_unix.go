//go:build !windows

package filepanel

import "golang.org/x/sys/unix"

// getFreeSpace returns the space available to unprivileged users on the filesystem of path.
func getFreeSpace(path string) (uint64, bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, false
	}
	//nolint:unconvert,gosec // Field types differ between platforms, and they are not negative
	return uint64(st.Bavail) * uint64(st.Bsize), true
}
