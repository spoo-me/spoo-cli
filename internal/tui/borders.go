package tui

import (
	lipgloss "charm.land/lipgloss/v2"
)

// Border glyph connectivity, as N/S/E/W stub bits. When panels share
// border lines (composeBody overlaps them by one cell), the winning
// box's corner glyphs are wrong at the seams — a ╭ where a ┬ belongs.
// healBorders fixes every such junction by looking at which neighbors
// point at each border cell.

const (
	stubN = 1 << iota
	stubS
	stubE
	stubW
)

var borderStubs = map[string]uint8{
	"─": stubE | stubW,
	"│": stubN | stubS,
	"╭": stubS | stubE,
	"╮": stubS | stubW,
	"╰": stubN | stubE,
	"╯": stubN | stubW,
	"├": stubN | stubS | stubE,
	"┤": stubN | stubS | stubW,
	"┬": stubS | stubE | stubW,
	"┴": stubN | stubE | stubW,
	"┼": stubN | stubS | stubE | stubW,
}

var stubGlyphs = map[uint8]string{
	stubE | stubW:                 "─",
	stubN | stubS:                 "│",
	stubS | stubE:                 "╭",
	stubS | stubW:                 "╮",
	stubN | stubE:                 "╰",
	stubN | stubW:                 "╯",
	stubN | stubS | stubE:         "├",
	stubN | stubS | stubW:         "┤",
	stubS | stubE | stubW:         "┬",
	stubN | stubE | stubW:         "┴",
	stubN | stubS | stubE | stubW: "┼",
}

// healBorders repairs junction glyphs in a composed layout: any border
// cell gains stubs toward neighboring border cells that point back at
// it, and is redrawn as the matching tee/cross. Styles are untouched,
// so a focused panel's colored ring keeps its hue at the joins.
func healBorders(s string, width, height int) string {
	canvas := lipgloss.NewCanvas(width, height)
	canvas.Compose(lipgloss.NewLayer(s))

	at := func(x, y int) uint8 {
		if x < 0 || y < 0 || x >= width || y >= height {
			return 0
		}
		if cell := canvas.CellAt(x, y); cell != nil {
			return borderStubs[cell.Content]
		}
		return 0
	}

	type fix struct {
		x, y  int
		glyph string
	}
	var fixes []fix
	for y := range height {
		for x := range width {
			cell := canvas.CellAt(x, y)
			if cell == nil {
				continue
			}
			own, ok := borderStubs[cell.Content]
			if !ok {
				continue
			}
			conn := own
			if at(x, y-1)&stubS != 0 {
				conn |= stubN
			}
			if at(x, y+1)&stubN != 0 {
				conn |= stubS
			}
			if at(x+1, y)&stubW != 0 {
				conn |= stubE
			}
			if at(x-1, y)&stubE != 0 {
				conn |= stubW
			}
			if g, ok := stubGlyphs[conn]; ok && conn != own {
				fixes = append(fixes, fix{x, y, g})
			}
		}
	}
	// apply after scanning so fixes don't cascade into each other
	for _, f := range fixes {
		canvas.CellAt(f.x, f.y).Content = f.glyph
	}
	return canvas.Render()
}
