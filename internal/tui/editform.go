package tui

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
	"github.com/spoo-me/spoo-cli/internal/ui"
)

var editAliasRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,16}$`)

// edit field indices; statusField is the trailing non-text toggle.
const (
	fDest = iota
	fAlias
	fPassword
	fMaxClicks
	fExpires
	statusField
	editFieldCount
)

type editFieldMeta struct{ label, desc string }

var editMeta = [statusField]editFieldMeta{
	fDest:      {"Destination", ""},
	fAlias:     {"Alias", ""},
	fPassword:  {"Password", "blank keeps the current password"},
	fMaxClicks: {"Max clicks", "0 removes the limit; blank keeps it"},
	fExpires:   {"Expires", "72h · 2026-12-31 · epoch — blank keeps it"},
}

// editForm is the pre-filled link editor: a stack of fields in a
// bordered dialog. tab/↑↓ move (with wrap), enter saves, esc cancels.
type editForm struct {
	open   bool
	item   api.URLItem
	inputs [statusField]textinput.Model
	status string // "active" | "inactive"
	focus  int
	err    string
}

func newEditForm() editForm { return editForm{} }

// show builds the form pre-filled from it and focuses the first field.
func (e editForm) show(it api.URLItem) (editForm, tea.Cmd) {
	e = editForm{open: true, item: it, status: strings.ToLower(it.Status)}
	if e.status != "active" && e.status != "inactive" {
		e.status = "active" // blocked/expired aren't user-settable
	}
	prefill := [statusField]string{fDest: it.LongURL, fAlias: it.Alias}
	if it.MaxClicks != nil {
		prefill[fMaxClicks] = strconv.Itoa(*it.MaxClicks)
	}
	for i := range e.inputs {
		in := textinput.New()
		in.Prompt = "> "
		st := in.Styles()
		st.Cursor.Color = ui.Accent // not the default green
		in.SetStyles(st)
		in.SetValue(prefill[i])
		if i == fPassword {
			in.EchoMode = textinput.EchoPassword
			in.Placeholder = "unchanged"
		}
		e.inputs[i] = in
	}
	return e, e.inputs[fDest].Focus()
}

// update advances the form. done is true on save, aborted on cancel.
func (e editForm) update(msg tea.Msg) (editForm, tea.Cmd, bool, bool) {
	key, ok := msg.(tea.KeyPressMsg)
	if !ok {
		var cmd tea.Cmd
		if e.focus < statusField {
			e.inputs[e.focus], cmd = e.inputs[e.focus].Update(msg)
		}
		return e, cmd, false, false
	}

	switch key.String() {
	case "ctrl+c", "esc":
		e.open = false
		return e, nil, false, true
	case "enter", "ctrl+s":
		if err := e.validate(); err != nil {
			e.err = err.Error()
			return e, nil, false, false
		}
		e.open = false
		return e, nil, true, false
	case "tab", "down":
		return e.refocus((e.focus + 1) % editFieldCount), nil, false, false
	case "shift+tab", "up":
		return e.refocus((e.focus + editFieldCount - 1) % editFieldCount), nil, false, false
	}

	if e.focus == statusField {
		switch key.String() {
		case "left", "right", "space", " ", "h", "l":
			if e.status == "active" {
				e.status = "inactive"
			} else {
				e.status = "active"
			}
		}
		return e, nil, false, false
	}

	var cmd tea.Cmd
	e.inputs[e.focus], cmd = e.inputs[e.focus].Update(msg)
	e.err = ""
	return e, cmd, false, false
}

// refocus moves the cursor to field i, blurring the rest.
func (e editForm) refocus(i int) editForm {
	e.err = ""
	for j := range e.inputs {
		e.inputs[j].Blur()
	}
	e.focus = i
	if i < statusField {
		e.inputs[i].Focus()
	}
	return e
}

func (e editForm) validate() error {
	u, err := url.Parse(e.inputs[fDest].Value())
	if err != nil || u.Scheme == "" || u.Host == "" {
		return errors.New("destination must be a full URL including https://")
	}
	if !editAliasRe.MatchString(e.inputs[fAlias].Value()) {
		return errors.New("alias must be 3-16 chars: letters, numbers, - and _")
	}
	if mc := e.inputs[fMaxClicks].Value(); mc != "" {
		if _, err := strconv.Atoi(mc); err != nil {
			return errors.New("max clicks must be a number")
		}
	}
	return nil
}

func (e editForm) view(width int) string {
	boxW := min(72, max(48, width-8))
	innerW := boxW - 6

	rows := []string{ui.Title.Render("✦ edit " + e.item.Alias), ""}
	for i := 0; i < statusField; i++ {
		label := ui.Dim.Bold(true).Render(editMeta[i].label)
		if e.focus == i {
			label = ui.Title.Render(editMeta[i].label)
		}
		rows = append(rows, label)
		if d := editMeta[i].desc; d != "" {
			rows = append(rows, ui.Dim.Render(d))
		}
		e.inputs[i].SetWidth(innerW - 2)
		rows = append(rows, e.inputs[i].View())
		rows = append(rows, "")
	}

	// status toggle
	statusLabel := ui.Dim.Bold(true).Render("Status")
	if e.focus == statusField {
		statusLabel = ui.Title.Render("Status")
	}
	rows = append(rows, statusLabel, e.statusToggle(), "")

	if e.err != "" {
		rows = append(rows, ui.Err.Render("✗ "+e.err))
	} else {
		rows = append(rows, ui.KeyHint.Render("tab/↑↓ move · enter save · esc cancel"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Accent).
		Padding(0, 2).
		Width(boxW).
		Render(strings.Join(rows, "\n"))
}

// statusToggle renders active/inactive as a pill pair.
func (e editForm) statusToggle() string {
	on := lipgloss.NewStyle().Foreground(ui.Success).Bold(true)
	off := lipgloss.NewStyle().Foreground(ui.Danger).Bold(true)
	pick := func(label string, active bool, style lipgloss.Style) string {
		if active {
			return ui.Title.Render("● ") + style.Render(label)
		}
		return ui.Dim.Render("○ " + label)
	}
	return pick("active", e.status == "active", on) + "   " +
		pick("inactive", e.status == "inactive", off)
}

// changes returns the PATCH body for fields that differ from the
// original link. Status is upper-cased to the API's enum.
func (e editForm) changes() (map[string]any, error) {
	f := map[string]any{}
	if v := e.inputs[fDest].Value(); v != e.item.LongURL {
		f["long_url"] = v
	}
	if v := e.inputs[fAlias].Value(); v != e.item.Alias {
		f["alias"] = v
	}
	if v := e.inputs[fPassword].Value(); v != "" {
		f["password"] = v
	}
	if mc := e.inputs[fMaxClicks].Value(); mc != "" {
		n, _ := strconv.Atoi(mc)
		cur := 0
		if e.item.MaxClicks != nil {
			cur = *e.item.MaxClicks
		}
		if n != cur {
			f["max_clicks"] = n
		}
	}
	if exp := e.inputs[fExpires].Value(); exp != "" {
		v, err := api.ParseExpiry(exp, time.Now())
		if err != nil {
			return nil, err
		}
		f["expire_after"] = v
	}
	if e.status != strings.ToLower(e.item.Status) {
		f["status"] = strings.ToUpper(e.status) // API wants ACTIVE / INACTIVE
	}
	return f, nil
}

// summary lists the pending changes for the confirmation dialog.
func (e editForm) summary(changes map[string]any) []string {
	label := map[string]string{
		"long_url": "destination", "alias": "alias", "password": "password",
		"max_clicks": "max clicks", "expire_after": "expires", "status": "status",
	}
	order := []string{"long_url", "alias", "password", "max_clicks", "expire_after", "status"}
	var lines []string
	for _, k := range order {
		v, ok := changes[k]
		if !ok {
			continue
		}
		shown := fmt.Sprintf("%v", v)
		if k == "password" {
			shown = "••••••"
		}
		lines = append(lines, "  "+label[k]+" → "+truncateToWidth(shown, 40))
	}
	return lines
}
