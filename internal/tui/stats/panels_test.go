package stats

import (
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/zalando/go-keyring"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/auth"
)

var ansiRe = regexp.MustCompile("\x1b\\[[0-9;]*m")

// TestPanelRowsAligned renders a panel whose values span widths (132k …
// 8.6k) and asserts every line is the same display width — the bar +
// dotted leader + compact count + percentage must always line up, with
// no row's bar touching its number.
func TestPanelRowsAligned(t *testing.T) {
	keyring.MockInit()
	client := api.New("http://x", auth.NewStore(t.TempDir()))
	m := New(client, "", "all", "")
	resp := &api.StatsResponse{
		Scope:   "all",
		Summary: api.StatsSummary{TotalClicks: 287558},
		Metrics: map[string][]map[string]any{
			"clicks_by_browser": {
				{"browser": "Chrome", "clicks": 131881.0},
				{"browser": "Safari", "clicks": 68807.0},
				{"browser": "Firefox", "clicks": 31537.0},
				{"browser": "Edge", "clicks": 25803.0},
				{"browser": "Brave", "clicks": 14335.0},
				{"browser": "Arc", "clicks": 8601.0},
			},
		},
	}
	next, _ := m.Update(statsLoadedMsg{res: resp})
	m = next.(Model)
	next, _ = m.Update(tea.WindowSizeMsg{Width: 200, Height: 50})
	m = next.(Model)

	clean := ansiRe.ReplaceAllString(m.panelView(1, 50, 6, 6), "")
	lines := strings.Split(clean, "\n")
	want := lipgloss.Width(lines[0])
	for i, ln := range lines {
		if got := lipgloss.Width(ln); got != want {
			t.Errorf("panel line %d width = %d, want %d:\n%q", i, got, want, ln)
		}
		// a coloured bar block (▀) must never sit directly against a digit
		runes := []rune(ln)
		for j := 1; j < len(runes); j++ {
			if runes[j-1] == '▀' && runes[j] >= '0' && runes[j] <= '9' {
				t.Errorf("panel line %d: bar touches the number: %q", i, ln)
			}
		}
	}
}
