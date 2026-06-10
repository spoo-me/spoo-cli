// Package tui contains the interactive Bubble Tea views.
package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/table"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

const defaultPageSize = 20

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

// LinksModel is the interactive link browser: a paginated table over
// GET /api/v1/urls with open/copy/toggle/delete actions, plus live
// search (/) and sort cycling (s).
type LinksModel struct {
	client      *api.Client
	apiBase     string
	openBrowser func(string) error
	copyText    func(string) error

	opts api.ListURLsOptions // current query: search, sort, status, page size

	tbl       table.Model
	searchBox textinput.Model
	searching bool
	page      *api.URLPage
	pageNo    int
	status    string // transient status-bar message
	pending   string // url id awaiting delete confirmation
	loading   bool
	err       error
	width     int
	height    int
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
		m.tbl.SetColumns(linkColumns(msg.Width))
		m.tbl.SetWidth(msg.Width)
		m.tbl.SetHeight(max(5, msg.Height-6))
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

// updateBrowse handles keys in normal browsing mode.
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
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
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
			if err := m.openBrowser(m.shortURL(it)); err != nil {
				m.status = ui.Err.Render("✗ " + err.Error())
			} else {
				m.status = ui.Dim.Render("opened " + m.shortURL(it))
			}
		}
		return m, nil
	case "c":
		if it := m.selected(); it != nil {
			if err := m.copyText(m.shortURL(it)); err != nil {
				m.status = ui.Err.Render("✗ " + err.Error())
			} else {
				m.status = ui.OK.Render("✓ copied " + m.shortURL(it))
			}
		}
		return m, nil
	case "t":
		if it := m.selected(); it != nil {
			return m, m.toggleStatus(it)
		}
		return m, nil
	case "d":
		it := m.selected()
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

	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
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
		created := it.CreatedAt
		if len(created) >= 10 {
			created = created[:10]
		}
		rows = append(rows, table.Row{
			it.Alias,
			it.LongURL,
			strconv.Itoa(it.TotalClicks),
			it.Status,
			created,
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
		b.WriteString(ui.KeyHint.Render("↑/↓ move · ←/→ pages · / search · s sort · o open · c copy · t toggle · d delete · r refresh · q quit"))
	}

	v := tea.NewView(b.String())
	v.AltScreen = true
	return v
}

// Err reports a fetch error that ended the session, if any.
func (m LinksModel) Err() error { return m.err }
