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

	out := styledTable(tsTree, 0, widths, header, rows, 0, 10, 40)
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

	out := styledTable(tsHeaderBand, 0, widths, header, rows, 0, 10, 40)
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

// Regression: with a rank column the label column shifts to index 1 —
// it must stay left-aligned next to the rank, and rows must never
// exceed the declared width (overflow wrapped rows and pushed the
// focus-mode sidebar off screen).
func TestRankColumnKeepsLabelAdjacentAndFits(t *testing.T) {
	innerW := 100
	widths := []int{3, max(10, innerW-26), 8, 8}
	header := []string{"#", "browser", "clicks", "share"}
	rows := [][]string{
		{"1", "Chrome", "49", "90.7%"},
		{"2", "Safari", "3", "5.6%"},
	}
	out := styledTable(tsTreeBand, 1, widths, header, rows, -1, 10, innerW)
	stripAnsi := func(s string) string {
		for strings.Contains(s, "\x1b[") {
			start := strings.Index(s, "\x1b[")
			end := strings.Index(s[start:], "m")
			s = s[:start] + s[start+end+1:]
		}
		return s
	}
	for i, line := range strings.Split(out, "\n") {
		plain := stripAnsi(line)
		if w := lgWidth(plain); w > innerW {
			t.Fatalf("line %d is %d cols, exceeds inner width %d:\n%q", i, w, innerW, plain)
		}
	}
	dataLine := stripAnsi(strings.Split(out, "\n")[1])
	rank := strings.Index(dataLine, "1")
	label := strings.Index(dataLine, "Chrome")
	if label-rank > 6 {
		t.Fatalf("label drifted %d cols from rank — not left-aligned:\n%q", label-rank, dataLine)
	}
}

func lgWidth(s string) int { return len([]rune(s)) }
