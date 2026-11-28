package metadata

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/yorukot/superfile/src/internal/common"
)

func TestPgUpDown(t *testing.T) {
	tenItems := Metadata{
		data: make([][2]string, 10),
	}
	testdata := []struct {
		name                string
		m                   Model
		pageScrollSize      int // value forced into common.Config.PageScrollSize
		pageDown            bool
		expectedRenderIndex int
	}{
		{
			name:                "Page down from top",
			m:                   Model{metadata: tenItems, renderIndex: 0},
			pageScrollSize:      3,
			pageDown:            true,
			expectedRenderIndex: 3,
		},
		{
			name:                "Page down near end wraps around",
			m:                   Model{metadata: tenItems, renderIndex: 8},
			pageScrollSize:      3,
			pageDown:            true,
			expectedRenderIndex: 1, // (8 + 3) % 10
		},
		{
			name:                "Page up from middle",
			m:                   Model{metadata: tenItems, renderIndex: 5},
			pageScrollSize:      3,
			pageDown:            false,
			expectedRenderIndex: 2,
		},
		{
			name:                "Page up near beginning wraps around",
			m:                   Model{metadata: tenItems, renderIndex: 1},
			pageScrollSize:      3,
			pageDown:            false,
			expectedRenderIndex: 8, // (1 - 3 + 10) % 10
		},
		{
			name:                "Page down on empty panel stays put",
			m:                   Model{metadata: Metadata{data: nil}, renderIndex: 0},
			pageScrollSize:      3,
			pageDown:            true,
			expectedRenderIndex: 0,
		},
		{
			name:                "Falls back to visible height when config unset",
			m:                   Model{metadata: tenItems, renderIndex: 0, height: 5},
			pageScrollSize:      0, // borderSize is 2 -> page size becomes 5 - 2 = 3
			pageDown:            true,
			expectedRenderIndex: 3,
		},
	}

	origPageScrollSize := common.Config.PageScrollSize
	t.Cleanup(func() { common.Config.PageScrollSize = origPageScrollSize })

	for _, tt := range testdata {
		t.Run(tt.name, func(t *testing.T) {
			common.Config.PageScrollSize = tt.pageScrollSize
			if tt.pageDown {
				tt.m.PgDown()
			} else {
				tt.m.PgUp()
			}
			assert.Equal(t, tt.expectedRenderIndex, tt.m.renderIndex)
		})
	}
}

func TestScrollStopsWhenLastLineVisible(t *testing.T) {
	// 10 rows, 3 visible lines (height 5 minus borders), so the last first-visible row is 7
	m := Model{metadata: Metadata{data: make([][2]string, 10)}, height: 5}
	for range 7 {
		m.ListDown()
	}
	assert.Equal(t, 7, m.renderIndex)
	m.ListDown()
	assert.Equal(t, 0, m.renderIndex, "wraps after the last position instead of scrolling past the end")

	m.renderIndex = 9
	m.ResetRenderIfInvalid()
	assert.Equal(t, 0, m.renderIndex)
}

func TestPermissionSelectionScrollsIntoView(t *testing.T) {
	data := make([][2]string, 10)
	data[8] = [2]string{keyPermissions, "rwxr-xr-x / 755"}
	m := Model{metadata: NewMetadata(data, "", ""), height: 5}
	m.metadata.hasMode, m.metadata.mode = true, 0o755

	m.MovePermissionSelection(1)
	assert.Equal(t, 6, m.renderIndex, "row 8 is the last of 3 visible lines")
	m.renderIndex = 9
	m.MovePermissionSelection(1)
	assert.Equal(t, 7, m.renderIndex, "scrolling up shows the row, within the allowed range")
}
