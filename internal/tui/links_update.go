package tui

import (
	"context"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

func (m LinksModel) fetch(pageNo int) tea.Cmd {
	opts := m.opts
	opts.Page = pageNo
	client := m.client
	return func() tea.Msg {
		page, err := client.ListURLs(context.Background(), opts)
		return pageMsg{page: page, err: err}
	}
}

func (m LinksModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.helper.SetWidth(msg.Width)
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
		m.syncPager()
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
		if m.edit.open {
			return m.updateEdit(msg)
		}
		if m.confirm.open {
			return m.updateConfirm(msg)
		}
		if m.qrURL != "" {
			return m.updateQR(msg)
		}
		if m.searching {
			return m.updateSearch(msg)
		}
		return m.updateBrowse(msg)
	}
	if m.edit.open { // form blink ticks, validation cmds
		return m.updateEdit(msg)
	}

	var cmd tea.Cmd
	m.tbl, cmd = m.tbl.Update(msg)
	return m, cmd
}

// updateEdit drives the embedded edit form; on submit it stages the
// changed fields and pops a save confirmation.
func (m LinksModel) updateEdit(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var done, aborted bool
	m.edit, cmd, done, aborted = m.edit.update(msg)
	if aborted {
		return m, cmd
	}
	if done {
		changes, err := m.edit.changes()
		if err != nil {
			m.status = ui.Err.Render("✗ " + err.Error())
			return m, cmd
		}
		if len(changes) == 0 {
			m.status = ui.Dim.Render("no changes")
			return m, cmd
		}
		m.pendingPATCH = changes
		it := m.edit.item
		m.confirm = m.confirm.askSimple("save", it.ID, "Save changes to "+it.Alias+"?", m.edit.summary(changes))
	}
	return m, cmd
}

// updateConfirm resolves the shared confirmation dialog and runs the
// tagged action when the user commits.
func (m LinksModel) updateConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	var confirmed bool
	var cmd tea.Cmd
	m.confirm, confirmed, cmd = m.confirm.handle(msg)
	if !confirmed {
		return m, cmd
	}
	switch m.confirm.tag {
	case "save":
		patch := m.pendingPATCH
		m.pendingPATCH = nil
		m.status = ui.Dim.Render("saving…")
		return m, m.applyPATCH(m.confirm.tagID, patch)
	case "delete":
		m.status = ui.Dim.Render("deleting…")
		return m, m.deleteURL(m.confirm.tagID)
	}
	return m, nil
}

// applyPATCH sends the staged edit and reports the outcome.
func (m LinksModel) applyPATCH(id string, fields map[string]any) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		_, err := client.UpdateURL(context.Background(), id, fields)
		return actionMsg{note: "link updated", err: err}
	}
}

// deleteURL removes a link by id and reports the outcome.
func (m LinksModel) deleteURL(id string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		err := client.DeleteURL(context.Background(), id)
		return actionMsg{note: "link deleted", err: err}
	}
}

// updateQR handles keys while the QR dialog is up.
func (m LinksModel) updateQR(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "c":
		if err := m.copyText(m.qrURL); err != nil {
			m.status = ui.Err.Render("✗ copy failed: " + err.Error())
		} else {
			m.status = ui.OK.Render("✓ copied " + m.qrURL)
		}
		m.qrURL = ""
	default: // any other key dismisses
		m.qrURL = ""
	}
	return m, nil
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
	// action keys return here and are never forwarded to the table —
	// its default keymap also binds some of them (e.g. 'd' is half
	// page down)
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
	case "e":
		if it := m.selected(); it != nil {
			var cmd tea.Cmd
			m.edit, cmd = m.edit.show(*it)
			return m, cmd
		}
		return m, nil
	case "Q", "shift+q":
		if it := m.selected(); it != nil {
			m.qrURL = m.shortURL(it)
		}
		return m, nil
	case "?":
		m.helper.ShowAll = !m.helper.ShowAll
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
		if it := m.selected(); it != nil {
			m.confirm = m.confirm.askPhrase("delete", it.ID,
				"Delete "+it.Alias+"?",
				[]string{ui.Dim.Render("  → " + truncateToWidth(it.LongURL, 44)), "", ui.Dim.Render("  This can't be undone.")},
				it.Alias)
			return m, nil
		}
		return m, nil
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
		// the endpoint defaults to a 7-day window; ask for the maximum
		from := time.Now().UTC().AddDate(0, 0, -api.MaxRangeDays).Format(time.RFC3339)
		res, err := client.Stats(context.Background(), api.StatsQuery{
			Scope:     "all",
			ShortCode: alias,
			StartDate: from,
			GroupBy:   []string{"time", "browser", "os", "country", "referrer"},
		})
		return statsMsg{alias: alias, res: res, err: err}
	}
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
