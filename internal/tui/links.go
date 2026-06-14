package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

// qrView is the QR dialog box; the host overlays it via overlayCenter.
func (m LinksModel) qrView() string {
	body := []string{
		ui.Title.Render("✦ " + m.qrURL),
		"",
		ui.QR(m.qrURL, false),
		"",
		ui.KeyHint.Render("c copy url · any key closes"),
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Accent).
		Padding(0, 2).
		Render(strings.Join(body, "\n"))
}

func (m LinksModel) View() tea.View {
	var b strings.Builder

	title := ui.Title.Render("spoo links")
	if m.page != nil && m.pager.TotalPages > 1 {
		title += ui.Dim.Render("  ") + m.pager.View()
	}
	if m.page != nil {
		title += ui.Dim.Render(fmt.Sprintf("  %d total", m.page.Total))
	}
	title += ui.Dim.Render("  sort: " + strings.ReplaceAll(m.opts.SortBy, "_", " "))
	if m.opts.Status != "" {
		title += ui.Dim.Render("  status: " + m.opts.Status)
	}
	if m.opts.Search != "" && !m.searching {
		title += ui.Dim.Render("  search: " + m.opts.Search)
	}
	b.WriteString(title + "\n\n")

	switch {
	case m.loading && m.page == nil:
		b.WriteString(ui.Dim.Render("loading…") + "\n")
	case m.page != nil && len(m.page.Items) == 0:
		b.WriteString(ui.Dim.Render("no links match — create one with `spoo shorten`") + "\n")
	case m.splitActive():
		_, dw := m.splitWidths()
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			m.tbl.View(), "  ", m.detailView(dw)) + "\n")
	case m.showDetail:
		b.WriteString(m.detailView(m.width-4) + "\n")
	default:
		b.WriteString(m.tbl.View() + "\n")
	}

	switch {
	case m.searching:
		b.WriteString(ui.Title.Render("/") + m.searchBox.View() + "\n")
		b.WriteString(ui.KeyHint.Render("enter apply · esc cancel"))
	default:
		if m.status != "" {
			b.WriteString(m.status + "\n")
		}
		b.WriteString(m.helper.View(linksKeys{}))
	}

	content := b.String()
	w, h := max(60, m.width), max(20, m.height)
	switch {
	case m.edit.open:
		content = overlayCenter(content, m.edit.view(w), w, h)
	case m.confirm.open:
		content = overlayCenter(content, m.confirm.view(), w, h)
	case m.qrURL != "":
		content = overlayCenter(content, m.qrView(), w, max(h, lipgloss.Height(m.qrView())+2))
	}
	v := tea.NewView(content)
	v.AltScreen = true
	return v
}

// detailView renders the selected link's full record at the given width.
func (m LinksModel) detailView(width int) string {
	it := m.selected()
	box := ui.Box.Width(max(40, width))
	if it == nil {
		return box.Render(ui.Dim.Render("nothing selected"))
	}
	label := func(s string) string { return ui.Dim.Render(fmt.Sprintf("%-14s", s)) }
	yesNo := func(b bool) string {
		if b {
			return "yes"
		}
		return "no"
	}
	statusLine := ui.OK.Render(it.Status)
	if it.Status != "ACTIVE" {
		statusLine = ui.Err.Render(it.Status)
	}
	// wrap long URLs so they stay inside the box, aligned past the label
	wrap := lipgloss.NewStyle().Width(max(24, width-20))
	field := func(name, value string) string {
		return lipgloss.JoinHorizontal(lipgloss.Top, label(name), wrap.Render(value))
	}

	lines := []string{
		ui.Title.Render(it.Alias) + "  " + statusLine,
		"",
		field("short url", m.shortURL(it)),
		field("destination", it.LongURL),
		"",
		label("clicks") + strconv.Itoa(it.TotalClicks),
		label("created") + isoDate(it.CreatedAt),
		label("last click") + orNever(isoDate(it.LastClick)),
		"",
		label("password") + yesNo(it.PasswordSet),
		label("private stats") + yesNo(it.PrivateStats),
		label("block bots") + yesNo(it.BlockBots),
	}
	if it.MaxClicks != nil {
		lines = append(lines, label("max clicks")+strconv.Itoa(*it.MaxClicks))
	}
	if it.ExpireAfter != nil {
		lines = append(lines, label("expires")+time.Unix(*it.ExpireAfter, 0).UTC().Format("2006-01-02 15:04 MST"))
	}
	if it.Domain != "" {
		lines = append(lines, label("domain")+it.Domain)
	}
	lines = append(lines, "", ui.Title.Render("analytics"))
	lines = append(lines, m.analyticsLines(it.Alias, label, width)...)
	return box.Render(strings.Join(lines, "\n"))
}

// analyticsLines renders the per-link stats section of the detail pane
// from the debounced cache; before the fetch lands it shows a loader.
func (m LinksModel) analyticsLines(alias string, label func(string) string, width int) []string {
	e, ok := m.stats[alias]
	if !ok {
		return []string{ui.Dim.Render("loading…")}
	}
	if e.err != nil || e.res == nil {
		return []string{ui.Dim.Render("unavailable")}
	}
	res := e.res
	if res.Summary.TotalClicks == 0 {
		return []string{ui.Dim.Render(fmt.Sprintf("no clicks in the last %d days", api.MaxRangeDays))}
	}
	total := float64(res.Summary.TotalClicks)
	unique := fmt.Sprintf("%d of %d clicks", res.Summary.UniqueClicks, res.Summary.TotalClicks)
	if rate, ok := res.ComputedMetrics["unique_click_rate"]; ok {
		unique += ui.Dim.Render(fmt.Sprintf(" (%.0f%%)", rate))
	}
	return []string{
		label("trend (90d)") + miniSpark(res.Points("time", "clicks"), max(20, width-24)),
		label("unique") + unique,
		label("avg redirect") + fmt.Sprintf("%.0fms", res.Summary.AvgRedirectionTime),
		label("top browser") + topOf(res, "browser", total, nil),
		label("top os") + topOf(res, "os", total, nil),
		label("top country") + topOf(res, "country", total, ui.CountryLabel),
		label("top referrer") + topOf(res, "referrer", total, nil),
	}
}

// topOf names the dominant label of a dimension with its share; format
// optionally decorates the label (e.g. country flag emoji).
func topOf(res *api.StatsResponse, dimension string, total float64, format func(string) string) string {
	pts := res.Points(dimension, "clicks")
	if len(pts) == 0 {
		return "—"
	}
	best := pts[0]
	for _, p := range pts[1:] {
		if p.Value > best.Value {
			best = p
		}
	}
	name := best.Label
	if format != nil {
		name = format(name)
	}
	if total > 0 {
		return fmt.Sprintf("%s %s", name, ui.Dim.Render(fmt.Sprintf("(%.0f%%)", best.Value/total*100)))
	}
	return name
}
