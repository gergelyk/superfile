package metadata

import "time"

// Spacing between Key and Value while rendering
const keyValueSpacing = " : "
const keyValueSpacingLen = 3

const fileStatErrorMsg = "Cannot load file stats"
const linkFileBrokenMsg = "Link file is broken!"
const etFetchErrorMsg = "Errors while fetching metadata via exiftool"
const dirSizeUnfocusedMsg = "(focus)"
const selectionCalculatingMsg = "calculating..."

// Timeout for the external `file` command
const fileCommandTimeout = 2 * time.Second

const keyName = "Name"
const keyFileType = "File Type"
const keySize = "Size"
const keySelectedSize = "Selected Size"
const keyDataModified = "Date Modified"
const keyDataAccessed = "Date Accessed"
const keyPermissions = "Permissions"
const keyMd5Checksum = "MD5Checksum"
const keyOwnerGroup = "Owner / Group"

// rootOwner is the name of the superuser, which is highlighted
const rootOwner = "root"
const keyAttributes = "Attributes"
const keySymlink = "Symlink"
const keyArchitecture = "Architecture"
const borderSize = 2

// Length of the "2006-01-02 15:04:05" part of a date
const dateHighlightLen = 19

// Number of permission bits (rwx for user, group and other)
const permBitCount = 9

// Cache configuration
const defaultCacheSize = 300
const defaultCacheExpiration = 5 * time.Minute

// Metadata field priority for display ordering. Other fields are sorted by name after these.
// Size and the selection size are always moved to the end while displaying.
var sortPriority = func() map[string]int { //nolint: gochecknoglobals // This is effectively const.
	order := []string{
		keyName, keyFileType, keyPermissions, keyAttributes, keyDataModified, keyDataAccessed,
		keyOwnerGroup, keySymlink, keyArchitecture,
	}
	res := make(map[string]int, len(order))
	for i, key := range order {
		res[key] = i
	}
	return res
}()
