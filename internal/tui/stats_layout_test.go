package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestPanelGridGeometryIsStable pins the breakdown grid's horizontal
// geometry (columns, per-panel x/width) so the renderer and mouse
// hit-testing can't silently drift when they share a layout source.
func TestPanelGridGeometryIsStable(t *testing.T) {
	cases := []struct {
		width    int
		wantCols int
		wantX    []int // x of first-row panels (items 1..)
		wantW    []int // width of first-row panels
	}{
		{160, 3, []int{0, 54, 108}, []int{53, 53, 52}},
		{100, 2, []int{0, 51}, []int{50, 49}},
		{80, 1, []int{0}, []int{80}},
	}
	for _, tc := range cases {
		m := newStatsModel(t, "")
		next, _ := m.Update(tea.WindowSizeMsg{Width: tc.width, Height: 50})
		m = next.(StatsModel)

		if got := m.gridCols(); got != tc.wantCols {
			t.Fatalf("width %d: gridCols=%d, want %d", tc.width, got, tc.wantCols)
		}

		byItem := map[int]hitRegion{}
		for _, r := range m.hitRegions() {
			byItem[r.item] = r
		}
		for n := range tc.wantW {
			r, ok := byItem[n+1]
			if !ok {
				t.Fatalf("width %d: no region for panel item %d", tc.width, n+1)
			}
			if r.x != tc.wantX[n] || r.w != tc.wantW[n] {
				t.Errorf("width %d panel item %d: x=%d w=%d, want x=%d w=%d",
					tc.width, n+1, r.x, r.w, tc.wantX[n], tc.wantW[n])
			}
		}
	}
}
