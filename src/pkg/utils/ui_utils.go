package utils

const CntFooterPanels = 3

const BorderPaddingForFooter = 2

// Including borders
func FullFooterHeight(footerHeight int, toggleFooter bool) int {
	if toggleFooter {
		return footerHeight + BorderPaddingForFooter
	}
	return 0
}

// Full width bar at the very bottom of the screen, showing focused panel path
const PathBarHeight = 1
