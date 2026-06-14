// Package tui contains the interactive Bubble Tea views.
package links

import (
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/paginator"
	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/tui/kit"
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

// Model is the interactive link browser: a paginated table over
// GET /api/v1/urls with open/copy/toggle/delete actions, live search
// (/), sort cycling (s), and a master-detail pane (enter).
type Model struct {
	client      *api.Client
	apiBase     string
	openBrowser func(string) error
	copyText    func(string) error

	opts api.ListURLsOptions // current query: search, sort, status, page size

	tbl          table.Model
	pager        paginator.Model
	searchBox    textinput.Model
	searching    bool
	edit         editForm       // 'e' opens the pre-filled link editor
	confirm      confirmDialog  // shared save/delete confirmation
	pendingPATCH map[string]any // edit changes awaiting confirmation
	helper       help.Model     // ? flips between short and full key help
	qrURL        string         // non-empty: the QR dialog is up for this URL
	showDetail   bool           // detail pane open; it always reflects the selected row
	stats        map[string]statsEntry
	statsSeq     int // bumped on selection change; stale debounce ticks no-op
	page         *api.URLPage
	pageNo       int
	status       string // transient status-bar message
	loading      bool
	err          error
	width        int
	height       int
}

func New(client *api.Client, apiBase string, opts api.ListURLsOptions, openBrowser, copyText func(string) error) Model {
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
	pager := paginator.New()
	pager.Type = paginator.Dots
	pager.PerPage = opts.PageSize
	pager.ActiveDot = lipgloss.NewStyle().Foreground(ui.Accent).Render("●")
	pager.InactiveDot = ui.Dim.Render("○")
	return Model{
		client:      client,
		apiBase:     apiBase,
		openBrowser: openBrowser,
		copyText:    copyText,
		opts:        opts,
		tbl:         tbl,
		pager:       pager,
		searchBox:   search,
		edit:        newEditForm(),
		confirm:     newConfirmDialog(),
		helper:      kit.NewHelp(),
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
func (m Model) splitActive() bool {
	return m.showDetail && m.width >= splitMinWidth
}

// splitWidths returns the table and detail-pane widths for the split layout.
func (m Model) splitWidths() (tableW, detailW int) {
	tableW = m.width * 11 / 20 // ~55% list, ~45% detail
	detailW = m.width - tableW - 2
	return tableW, detailW
}

// relayout resizes the table for the current width and detail state.
func (m *Model) relayout() {
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

// syncPager drives the (display-only) paginator from the server's
// page data: dots for a handful of pages, a 1/N readout beyond that.
func (m *Model) syncPager() {
	if m.page == nil {
		return
	}
	if m.page.PageSize > 0 {
		m.pager.PerPage = m.page.PageSize
	}
	m.pager.SetTotalPages(m.page.Total)
	m.pager.Page = m.page.Page - 1 // paginator is 0-indexed
	if m.pager.TotalPages > 12 {
		m.pager.Type = paginator.Arabic
	} else {
		m.pager.Type = paginator.Dots
	}
}

func (m Model) Init() tea.Cmd { return m.fetch(m.pageNo) }

func (m Model) selected() *api.URLItem {
	if m.page == nil || len(m.page.Items) == 0 {
		return nil
	}
	i := m.tbl.Cursor()
	if i < 0 || i >= len(m.page.Items) {
		return nil
	}
	return &m.page.Items[i]
}

func (m Model) shortURL(it *api.URLItem) string {
	if it.Domain != "" {
		return "https://" + it.Domain + "/" + it.Alias
	}
	return strings.TrimRight(m.apiBase, "/") + "/" + it.Alias
}

func nextSortField(current string) string {
	for i, f := range sortFields {
		if f == current {
			return sortFields[(i+1)%len(sortFields)]
		}
	}
	return sortFields[0]
}

func (m Model) rows() []table.Row {
	rows := make([]table.Row, 0, len(m.page.Items))
	for _, it := range m.page.Items {
		rows = append(rows, table.Row{
			it.Alias,
			it.LongURL,
			strconv.Itoa(it.TotalClicks),
			it.Status,
			kit.ISODate(it.CreatedAt),
		})
	}
	return rows
}

// Err reports a fetch error that ended the session, if any.
func (m Model) Err() error { return m.err }
