package stats

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// The link switcher ('g') re-targets the dashboard without leaving it:
// a fuzzy-filtered picker over your links, with "all links" on top.

const switcherRows = 8 // result rows visible at once

type linksListMsg struct {
	items []api.URLItem
	err   error
}

// openSwitcher pops the picker, fetching the link list on first use.
func (m Model) openSwitcher() (tea.Model, tea.Cmd) {
	if m.scope == "anon" {
		m.status = ui.Dim.Render("log in to switch links")
		return m, nil
	}
	m.switchMode = true
	m.switchSel = 0
	m.switchBox.SetValue("")
	cmds := []tea.Cmd{m.switchBox.Focus()}
	if m.switchAll == nil {
		client := m.client
		cmds = append(cmds, func() tea.Msg {
			page, err := client.ListURLs(context.Background(), api.ListURLsOptions{
				PageSize: 100, SortBy: "total_clicks",
			})
			if err != nil {
				return linksListMsg{err: err}
			}
			return linksListMsg{items: page.Items}
		})
	}
	return m, tea.Batch(cmds...)
}

// switchCandidates filters the cached list by the typed query.
func (m Model) switchCandidates() []api.URLItem {
	q := strings.ToLower(strings.TrimSpace(m.switchBox.Value()))
	if q == "" {
		return m.switchAll
	}
	var out []api.URLItem
	for _, it := range m.switchAll {
		if strings.Contains(strings.ToLower(it.Alias), q) ||
			strings.Contains(strings.ToLower(it.LongURL), q) {
			out = append(out, it)
		}
	}
	return out
}

// updateSwitcher handles keys while the picker is up.
func (m Model) updateSwitcher(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc":
		m.switchMode = false
		m.switchBox.Blur()
		return m, nil
	case "up", "ctrl+k":
		m.switchSel = max(0, m.switchSel-1)
		return m, nil
	case "down", "ctrl+j":
		m.switchSel = min(len(m.switchCandidates()), m.switchSel+1)
		return m, nil
	case "enter":
		cands := m.switchCandidates()
		target := "" // row 0 is "all links"
		if m.switchSel > 0 {
			if m.switchSel-1 >= len(cands) {
				return m, nil
			}
			target = cands[m.switchSel-1].Alias
		}
		m.switchMode = false
		m.switchBox.Blur()
		if target == m.target {
			return m, nil
		}
		// new subject: drill-downs and selections belong to the old one
		m.target = target
		m.filters = nil
		m.sel = map[int]int{}
		m.focus = 0
		m.focusItem = 0
		m.loading = true
		return m, m.fetch()
	}
	var cmd tea.Cmd
	m.switchBox, cmd = m.switchBox.Update(msg)
	m.switchSel = 0 // typing re-anchors to the top match
	return m, cmd
}

// switcherView is the picker box; the host overlays it via overlayCenter.
func (m Model) switcherView() string {
	row := func(label, note string, selected bool) string {
		marker, style := "  ", lipgloss.NewStyle()
		if selected {
			marker, style = ui.Title.Render("▸ "), ui.Title
		}
		line := marker + style.Render(kit.PadToWidth(kit.TruncateToWidth(label, 28), 28))
		if note != "" {
			line += " " + ui.Dim.Render(note)
		}
		return line
	}

	lines := []string{
		ui.Title.Render("✦ switch link"),
		"",
		ui.Dim.Render("find ") + " " + m.switchBox.View(),
		"",
		row("all links", "account-wide", m.switchSel == 0),
	}
	cands := m.switchCandidates()
	switch {
	case m.switchAll == nil:
		lines = append(lines, ui.Dim.Render("  loading your links…"))
	case len(cands) == 0:
		lines = append(lines, ui.Dim.Render("  no links match"))
	default:
		// keep the selection in view
		start := max(0, min(m.switchSel-1-switcherRows+1, len(cands)-switcherRows))
		for i := start; i < min(len(cands), start+switcherRows); i++ {
			it := cands[i]
			lines = append(lines, row(it.Alias, fmt.Sprintf("%d clicks", it.TotalClicks), m.switchSel == i+1))
		}
		if len(cands) > switcherRows {
			lines = append(lines, ui.Dim.Render(fmt.Sprintf("  … %d links", len(cands))))
		}
	}
	lines = append(lines, "", ui.KeyHint.Render("↑/↓ choose · enter switch · esc cancel"))

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Accent).
		Padding(0, 2).
		Width(min(56, max(44, m.width-8))).
		Render(strings.Join(lines, "\n"))
}
