// Package tui contains the interactive Bubble Tea views.
package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

const (
	defaultPageSize = 20
	// splitMinWidth is the narrowest terminal that fits the side-by-side
	// master-detail layout; below it the detail pane goes full screen.
	splitMinWidth = 100
	// statsDebounce is how long the selection must rest before the pane
	// fetches that link's analytics — scrolling through rows costs zero
	// API calls; only the row you stop on is fetched (then cached).
	statsDebounce = 300 * time.Millisecond
)

// sortFields is the cycle order for the 's' key.
var sortFields = []string{"total_clicks", "created_at", "last_click"}

type pageMsg struct {
	page *api.URLPage
	err  error
}

type actionMsg struct {
	note string
	err  error
}

// statsTickMsg fires after the debounce delay; seq identifies which
// selection scheduled it so stale ticks are dropped.
type statsTickMsg struct {
	seq int
}

type statsMsg struct {
	alias string
	res   *api.StatsResponse
	err   error
}

type statsEntry struct {
	res *api.StatsResponse
	err error
}

// LinksModel is the interactive link browser: a paginated table over
// GET /api/v1/urls with open/copy/toggle/delete actions, live search
// (/), sort cycling (s), and a master-detail pane (enter).
type LinksModel struct {
	client      *api.Client
	apiBase     string
	openBrowser func(string) error
	copyText    func(string) error

	opts api.ListURLsOptions // current query: search, sort, status, page size

	tbl        table.Model
	searchBox  textinput.Model
	searching  bool
	showDetail bool // detail pane open; it always reflects the selected row
	stats      map[string]statsEntry
	statsSeq   int // bumped on selection change; stale debounce ticks no-op
	page       *api.URLPage
	pageNo     int
	status     string // transient status-bar message
	pending    string // url id awaiting delete confirmation
	loading    bool
	err        error
	width      int
	height     int
}

func NewLinks(client *api.Client, apiBase string, opts api.ListURLsOptions, openBrowser, copyText func(string) error) LinksModel {
	if opts.PageSize <= 0 {
		opts.PageSize = defaultPageSize
	}
	if opts.SortBy == "" {
		opts.SortBy = sortFields[0]
	}
	tbl := table.New(
		table.WithColumns(linkColumns(80)),
		table.WithFocused(true),
		table.WithHeight(opts.PageSize),
	)
	search := textinput.New()
	search.Placeholder = "search alias or destination…"
	return LinksModel{
		client:      client,
		apiBase:     apiBase,
		openBrowser: openBrowser,
		copyText:    copyText,
		opts:        opts,
		tbl:         tbl,
		searchBox:   search,
		stats:       map[string]statsEntry{},
		pageNo:      max(1, opts.Page),
		loading:     true,
		width:       80,
	}
}

func linkColumns(width int) []table.Column {
	// Fixed columns get what they need; the destination takes the rest.
	rest := max(20, width-16-8-10-12-6)
	return []table.Column{
		{Title: "Alias", Width: 16},
		{Title: "Destination", Width: rest},
		{Title: "Clicks", Width: 8},
		{Title: "Status", Width: 10},
		{Title: "Created", Width: 12},
	}
}

// splitActive reports whether the side-by-side layout is in effect.
func (m LinksModel) splitActive() bool {
	return m.showDetail && m.width >= splitMinWidth
}

// splitWidths returns the table and detail-pane widths for the split layout.
func (m LinksModel) splitWidths() (tableW, detailW int) {
	tableW = m.width * 11 / 20 // ~55% list, ~45% detail
	detailW = m.width - tableW - 2
	return tableW, detailW
}

// relayout resizes the table for the current width and detail state.
func (m *LinksModel) relayout() {
	tw := m.width
	if m.splitActive() {
		tw, _ = m.splitWidths()
	}
	m.tbl.SetColumns(linkColumns(tw))
	m.tbl.SetWidth(tw)
	if m.height > 0 {
		m.tbl.SetHeight(max(5, m.height-6))
	}
}

func (m LinksModel) Init() tea.Cmd { return m.fetch(m.pageNo) }

func (m LinksModel) fetch(pageNo int) tea.Cmd {
	opts := m.opts
	opts.Page = pageNo
	client := m.client
	return func() tea.Msg {
		page, err := client.ListURLs(context.Background(), opts)
		return pageMsg{page: page, err: err}
	}
}

func (m LinksModel) selected() *api.URLItem {
	if m.page == nil || len(m.page.Items) == 0 {
		return nil
	}
	i := m.tbl.Cursor()
	if i < 0 || i >= len(m.page.Items) {
		return nil
	}
	return &m.page.Items[i]
}

func (m LinksModel) shortURL(it *api.URLItem) string {
	if it.Domain != "" {
		return "https://" + it.Domain + "/" + it.Alias
	}
	return strings.TrimRight(m.apiBase, "/") + "/" + it.Alias
}

func (m LinksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.relayout()
		return m, nil

	case pageMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, tea.Quit
		}
		m.page = msg.page
		m.pageNo = msg.page.Page
		m.tbl.SetRows(m.rows())
		m.tbl.GotoTop()
		m.status = ""
		return m, nil

	case actionMsg:
		if msg.err != nil {
			m.status = ui.Err.Render("✗ " + msg.err.Error())
			return m, nil
		}
		m.status = ui.OK.Render("✓ " + msg.note)
		return m, m.fetch(m.pageNo)

	case statsTickMsg:
		// only the most recent selection's tick survives the debounce
		if msg.seq != m.statsSeq || !m.showDetail {
			return m, nil
		}
		it := m.selected()
		if it == nil {
			return m, nil
		}
		if _, ok := m.stats[it.Alias]; ok {
			return m, nil
		}
		return m, m.fetchStats(it.Alias)

	case statsMsg:
		m.stats[msg.alias] = statsEntry{res: msg.res, err: msg.err}
		return m, nil

	case tea.KeyPressMsg:
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateBrowse(msg)
	}

	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

// updateSearch handles keys while the search box is focused.
func (m LinksModel) updateSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.searching = false
		m.searchBox.Blur()
		m.opts.Search = strings.TrimSpace(m.searchBox.Value())
		m.loading = true
		return m, m.fetch(1)
	case "esc":
		m.searching = false
		m.searchBox.Blur()
		m.searchBox.SetValue(m.opts.Search)
		return m, nil
	}
	var cmd tea.Cmd
	m.searchBox, cmd = m.searchBox.Update(msg)
	return m, cmd
}

// updateBrowse handles keys in browse mode. The detail pane mirrors the
// selection, so navigation stays live while it is open.
func (m LinksModel) updateBrowse(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	// any key other than a second 'd' cancels a pending delete
	if m.pending != "" && key != "d" {
		m.pending = ""
		m.status = ""
	}
	// action keys return here and are never forwarded to the table —
	// its default keymap also binds some of them (e.g. 'd' is half
	// page down, which would move the cursor mid-confirmation)
	switch key {
	case "ctrl+c":
		return m, tea.Quit
	case "q":
		return m, tea.Quit
	case "esc", "backspace":
		if m.showDetail {
			m.showDetail = false
			m.relayout()
			return m, nil
		}
		return m, tea.Quit
	case "enter":
		if m.showDetail {
			m.showDetail = false
			m.relayout()
			return m, nil
		}
		if m.selected() != nil {
			m.showDetail = true
			m.relayout()
			return m, m.scheduleStats()
		}
		return m, nil
	case "r":
		m.loading = true
		return m, m.fetch(m.pageNo)
	case "/":
		m.searching = true
		m.searchBox.SetValue(m.opts.Search)
		return m, m.searchBox.Focus()
	case "s":
		m.opts.SortBy = nextSortField(m.opts.SortBy)
		m.loading = true
		return m, m.fetch(1)
	case "right", "n":
		if m.page != nil && m.page.HasNext {
			m.loading = true
			return m, m.fetch(m.pageNo + 1)
		}
		return m, nil
	case "left", "p":
		if m.pageNo > 1 {
			m.loading = true
			return m, m.fetch(m.pageNo - 1)
		}
		return m, nil
	case "o":
		if it := m.selected(); it != nil {
			m.status = m.openStatus(it)
		}
		return m, nil
	case "c":
		if it := m.selected(); it != nil {
			m.status = m.copyStatus(it)
		}
		return m, nil
	case "t":
		if it := m.selected(); it != nil {
			return m, m.toggleStatus(it)
		}
		return m, nil
	case "d":
		return m.armOrConfirmDelete(m.selected())
	}

	prev := m.tbl.Cursor()
	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	if m.showDetail && m.tbl.Cursor() != prev {
		return m, tea.Batch(cmd, m.scheduleStats())
	}
	return m, cmd
}

// scheduleStats arms the debounce timer for the current selection.
func (m *LinksModel) scheduleStats() tea.Cmd {
	it := m.selected()
	if it == nil {
		return nil
	}
	if _, ok := m.stats[it.Alias]; ok {
		return nil // already loaded; the pane renders from cache
	}
	m.statsSeq++
	seq := m.statsSeq
	return tea.Tick(statsDebounce, func(time.Time) tea.Msg {
		return statsTickMsg{seq: seq}
	})
}

func (m LinksModel) fetchStats(alias string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		res, err := client.Stats(context.Background(), api.StatsQuery{
			Scope:     "all",
			ShortCode: alias,
			GroupBy:   []string{"time", "browser", "country", "referrer"},
		})
		return statsMsg{alias: alias, res: res, err: err}
	}
}

func (m LinksModel) armOrConfirmDelete(it *api.URLItem) (tea.Model, tea.Cmd) {
	if it == nil {
		return m, nil
	}
	if m.pending == it.ID {
		m.pending = ""
		return m, m.deleteURL(it)
	}
	m.pending = it.ID
	m.status = ui.Err.Render("delete "+it.Alias+"?") + ui.Dim.Render(" press d again to confirm")
	return m, nil
}

func (m LinksModel) openStatus(it *api.URLItem) string {
	if err := m.openBrowser(m.shortURL(it)); err != nil {
		return ui.Err.Render("✗ " + err.Error())
	}
	return ui.Dim.Render("opened " + m.shortURL(it))
}

func (m LinksModel) copyStatus(it *api.URLItem) string {
	if err := m.copyText(m.shortURL(it)); err != nil {
		return ui.Err.Render("✗ " + err.Error())
	}
	return ui.OK.Render("✓ copied " + m.shortURL(it))
}

func nextSortField(current string) string {
	for i, f := range sortFields {
		if f == current {
			return sortFields[(i+1)%len(sortFields)]
		}
	}
	return sortFields[0]
}

func (m LinksModel) toggleStatus(it *api.URLItem) tea.Cmd {
	client := m.client
	id, alias, status := it.ID, it.Alias, it.Status
	return func() tea.Msg {
		next := "INACTIVE"
		if status != "ACTIVE" {
			next = "ACTIVE"
		}
		_, err := client.UpdateURL(context.Background(), id, map[string]any{"status": next})
		return actionMsg{note: alias + " → " + next, err: err}
	}
}

func (m LinksModel) deleteURL(it *api.URLItem) tea.Cmd {
	client := m.client
	id, alias := it.ID, it.Alias
	return func() tea.Msg {
		err := client.DeleteURL(context.Background(), id)
		return actionMsg{note: "deleted " + alias, err: err}
	}
}

func (m LinksModel) rows() []table.Row {
	rows := make([]table.Row, 0, len(m.page.Items))
	for _, it := range m.page.Items {
		rows = append(rows, table.Row{
			it.Alias,
			it.LongURL,
			strconv.Itoa(it.TotalClicks),
			it.Status,
			isoDate(it.CreatedAt),
		})
	}
	return rows
}

func (m LinksModel) View() tea.View {
	var b strings.Builder

	title := ui.Title.Render("spoo links")
	if m.page != nil {
		title += ui.Dim.Render(fmt.Sprintf("  page %d · %d total", m.pageNo, m.page.Total))
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
		hint := "↑/↓ move · ←/→ pages · enter details · / search · s sort · o open · c copy · t toggle · d delete · r refresh · q quit"
		if m.showDetail {
			hint = "↑/↓ move · enter/esc close · o open · c copy · t toggle · d delete · q quit"
		}
		b.WriteString(ui.KeyHint.Render(hint))
	}

	v := tea.NewView(b.String())
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
		return []string{ui.Dim.Render("no clicks yet")}
	}
	total := float64(res.Summary.TotalClicks)
	return []string{
		label("trend") + miniSpark(res.Points("time", "clicks"), max(20, width-24)),
		label("unique") + fmt.Sprintf("%d of %d clicks", res.Summary.UniqueClicks, res.Summary.TotalClicks),
		label("top browser") + topOf(res, "browser", total),
		label("top country") + topOf(res, "country", total),
		label("top referrer") + topOf(res, "referrer", total),
	}
}

// miniSpark draws a compact sparkline of the most recent points.
func miniSpark(pts []api.MetricPoint, width int) string {
	if len(pts) > width {
		pts = pts[len(pts)-width:]
	}
	var maxV float64
	for _, p := range pts {
		maxV = max(maxV, p.Value)
	}
	if maxV == 0 {
		return ui.Dim.Render("flat")
	}
	var b strings.Builder
	for _, p := range pts {
		b.WriteRune(ui.SparkRunes[int(p.Value/maxV*float64(len(ui.SparkRunes)-1))])
	}
	return b.String()
}

// topOf names the dominant label of a dimension with its share.
func topOf(res *api.StatsResponse, dimension string, total float64) string {
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
	if total > 0 {
		return fmt.Sprintf("%s (%.0f%%)", best.Label, best.Value/total*100)
	}
	return best.Label
}

func isoDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func orNever(s string) string {
	if s == "" {
		return "never"
	}
	return s
}

// Err reports a fetch error that ended the session, if any.
func (m LinksModel) Err() error { return m.err }
