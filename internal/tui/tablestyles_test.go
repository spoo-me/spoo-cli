package tui

import (
	"strings"
	"testing"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// Regression: tree rows prepend a pre-styled branch; wrapping the line
// in the selection style must not be cut short by the branch's ANSI
// reset. The whole selected row renders as exactly one styled run.
func TestTreeSelectedRowFullyStyled(t *testing.T) {
	widths := []int{10, 6, 6}
	header := []string{"browser", "clicks", "share"}
	rows := [][]string{{"Chrome", "49", "91%"}, {"Safari", "3", "6%"}}

	out := styledTable(tsTree, widths, header, rows, 0, 10, 40)
	lines := strings.Split(out, "\n")
	selLine := lines[1] // header is line 0

	want := ui.Title.Render(" ├─ " + padToWidth("Chrome", 7) + " " + "    49" + " " + "   91%")
	if selLine != want {
		t.Fatalf("selected tree row not one styled run:\n got %q\nwant %q", selLine, want)
	}
	if !strings.Contains(selLine, "Chrome") {
		t.Fatal("selected row lost its label")
	}
}

// Unselected non-tree rows and selected ones must align (no stray
// leading space on the selection).
func TestSelectionDoesNotShiftColumns(t *testing.T) {
	widths := []int{10, 6, 6}
	header := []string{"browser", "clicks", "share"}
	rows := [][]string{{"Chrome", "49", "91%"}, {"Safari", "3", "6%"}}

	out := styledTable(tsHeaderBand, widths, header, rows, 0, 10, 40)
	lines := strings.Split(out, "\n")
	stripped := func(s string) string {
		for strings.Contains(s, "\x1b[") {
			start := strings.Index(s, "\x1b[")
			end := strings.Index(s[start:], "m")
			s = s[:start] + s[start+end+1:]
		}
		return s
	}
	sel, unsel := stripped(lines[1]), stripped(lines[2])
	if strings.Index(sel, "Chrome") != strings.Index(unsel, "Safari") {
		t.Fatalf("column shift between selected and unselected:\n%q\n%q", sel, unsel)
	}
}
