package tui

import (
	"image/color"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

const (
	panelTopN   = 6
	focusTopN   = 24 // rows shown for a panel promoted to focus mode
	twoColMin   = 96
	threeColMin = 140
	sidebarW    = 36 // focus-mode sidebar width
	autoEvery   = 30 * time.Second
)

// defaultWindow is the widest window the server allows — the silent
// server default is only 7 days, which hides most history.
var defaultWindow = timeWindow{span: api.MaxRangeDays * 24 * time.Hour, label: "90d"}

type panelDef struct{ key, title string }

// dashBarStyle is the bar style used across the dashboard (picked from
// a live A/B of six candidates).
const dashBarStyle = ui.BarUpperHalf

// panelColors gives every panel its own pastel hue (entity brand
// colors in colors.go override per row where known).
// the time chart's series duo: sky for clicks, pink for unique
var (
	chartClicks = lipgloss.NewStyle().Foreground(ui.Blue)
	chartUnique = lipgloss.NewStyle().Foreground(ui.Pink)
)

var panelColors = map[string]color.Color{
	"short_code": ui.Accent,
	"browser":    ui.Success,
	"os":         ui.Blue,
	"country":    ui.Yellow,
	"city":       ui.Pink,
	"referrer":   ui.Teal,
	"weekday":    ui.Accent,
}

type statsLoadedMsg struct {
	res  *api.StatsResponse
	prev *api.StatsResponse // previous window, for period-over-period deltas
	err  error
}

type exportDoneMsg struct {
	name string
	err  error
}

type autoTickMsg struct{}

type filterEntry struct {
	dim   string
	value string
}

// StatsModel is the full-screen analytics dashboard: overview with
// period deltas, a dual-series time chart, focusable breakdown panels
// with server-side drill-down, window paging, and a focus mode.
type StatsModel struct {
	client *api.Client
	target string // short code, or "" for account-wide
	scope  string // all | anon
	tz     string

	win      timeWindow
	offset   int // how many windows back in time ('[' / ']')
	metric   string
	filters  []filterEntry
	auto     bool
	showPrev bool // ghost the previous window on the time chart ('p')

	rangeMode bool // the 'T' range-expression strip is open
	rangeBox  textinput.Model
	rangeErr  string

	exportBox exportModal
	helper    help.Model // ? flips between short and full key help

	switchMode bool // the 'g' link picker is up
	switchBox  textinput.Model
	switchAll  []api.URLItem // fetched once, cached for the session
	switchSel  int           // 0 = "all links", 1.. = filtered items

	res      *api.StatsResponse
	prev     *api.StatsResponse
	fetchErr error
	loading  bool
	status   string
	focus    int             // index into panels()
	sel      map[int]int     // per-panel selection row
	tableOn  map[string]bool // panels currently in table view (by key)

	focusMode bool
	focusItem int // 0 = time chart, 1.. = panels()[focusItem-1]
	focusPane int // 0 = main view, 1 = sidebar (←/→ switches)

	width  int
	height int
}

func NewStats(client *api.Client, target, scope, tz string) StatsModel {
	rangeBox := textinput.New()
	rangeBox.Placeholder = "type a range…"
	rangeBox.SetWidth(36) // fits "2026-01-01 to 2026-02-15" with room; keeps the cheat-sheet column still
	switchBox := textinput.New()
	switchBox.Placeholder = "alias or destination…"
	switchBox.SetWidth(32)
	return StatsModel{
		client:    client,
		target:    target,
		scope:     scope,
		tz:        tz,
		win:       defaultWindow,
		rangeBox:  rangeBox,
		switchBox: switchBox,
		exportBox: newExportModal(),
		helper:    newHelp(),
		metric:    "clicks",
		sel:       map[int]int{},
		tableOn:   map[string]bool{},
		loading:   true,
		width:     100,
		height:    40,
	}
}

// panels returns the breakdown panels for the current view. Account-
// wide gets the drillable top-links leaderboard first; a single link
// gets the weekday distribution instead.
func (m StatsModel) panels() []panelDef {
	if m.target == "" {
		return []panelDef{
			{"short_code", "top links"},
			{"browser", "browsers"},
			{"os", "operating systems"},
			{"country", "countries"},
			{"city", "cities"},
			{"referrer", "referrers"},
		}
	}
	return []panelDef{
		{"browser", "browsers"},
		{"os", "operating systems"},
		{"country", "countries"},
		{"city", "cities"},
		{"referrer", "referrers"},
		{"weekday", "weekdays"},
	}
}

func (m StatsModel) Init() tea.Cmd { return m.fetch() }

// FetchErr reports a fetch error so the command can surface it on exit.
func (m StatsModel) FetchErr() error { return m.fetchErr }
