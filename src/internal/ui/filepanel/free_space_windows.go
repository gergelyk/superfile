//go:build windows

package filepanel

import "golang.org/x/sys/windows"

// getFreeSpace returns the space available to the current user on the volume of path.
func getFreeSpace(path string) (uint64, bool) {
	ptr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	var freeAvailable, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &freeAvailable, &total, &totalFree); err != nil {
		return 0, false
	}
	return freeAvailable, true
}
