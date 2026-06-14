package kit

import (
	lipgloss "charm.land/lipgloss/v2"
)

// backdropFg is the single flat gray the backdrop collapses to while
// a dialog is up — terminals can't blur, so dimming is the backdrop
// effect: every cell keeps its glyph but loses its color.
var backdropFg = lipgloss.Color("#3F4450")

// dimBackdrop repaints all of bg's cells in the backdrop gray.
func dimBackdrop(bg string, width, height int) string {
	canvas := lipgloss.NewCanvas(width, height)
	canvas.Compose(lipgloss.NewLayer(bg))
	for y := range height {
		for x := range width {
			if cell := canvas.CellAt(x, y); cell != nil {
				cell.Style.Fg = backdropFg
				cell.Style.Bg = nil
				cell.Style.Attrs = 0
			}
		}
	}
	return canvas.Render()
}

// Center composites fg over bg, centered. The background stays visible
// but drops to the backdrop gray so the dialog owns the eye.
func Center(bg, fg string, width, height int) string {
	return lipgloss.NewCompositor(
		lipgloss.NewLayer(dimBackdrop(bg, width, height)),
		lipgloss.NewLayer(fg).
			X(max(0, (width-lipgloss.Width(fg))/2)).
			Y(max(0, (height-lipgloss.Height(fg))/2)).
			Z(1),
	).Render()
}
