package tui

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"

	"github.com/spoo-me/spoo-cli/internal/api"
)

var editAliasRe = regexp.MustCompile(`^[A-Za-z0-9_-]{3,16}$`)

// editVals is heap-allocated so the huh form's bound pointers stay
// valid as the (value-type) bubbletea model is copied each update.
type editVals struct {
	longURL, alias, password, maxClicks, expires, status string
}

// editForm is the pre-filled link editor: a huh form over a link's
// editable properties. On completion the host reads changes() for the
// fields that actually differ and PATCHes them.
type editForm struct {
	open bool
	form *huh.Form
	vals *editVals
	item api.URLItem
}

func newEditForm() editForm { return editForm{} }

// show builds the form pre-filled from it.
func (e editForm) show(it api.URLItem) (editForm, tea.Cmd) {
	v := &editVals{
		longURL: it.LongURL,
		alias:   it.Alias,
		status:  strings.ToLower(it.Status),
	}
	if it.MaxClicks != nil {
		v.maxClicks = strconv.Itoa(*it.MaxClicks)
	}
	if v.status != "active" && v.status != "inactive" {
		v.status = "active" // blocked/expired aren't user-settable; default the toggle
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().Title("Destination").Value(&v.longURL).
				Validate(func(s string) error {
					u, err := url.Parse(s)
					if err != nil || u.Scheme == "" || u.Host == "" {
						return errors.New("enter a full URL including https://")
					}
					return nil
				}),
			huh.NewInput().Title("Alias").Value(&v.alias).
				Validate(func(s string) error {
					if !editAliasRe.MatchString(s) {
						return errors.New("3-16 chars: letters, numbers, - and _")
					}
					return nil
				}),
			huh.NewInput().Title("Password").
				Description("blank keeps the current password").
				EchoMode(huh.EchoModePassword).Value(&v.password),
			huh.NewInput().Title("Max clicks").
				Description("0 removes the limit; blank keeps it").
				Value(&v.maxClicks).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					if _, err := strconv.Atoi(s); err != nil {
						return errors.New("must be a number")
					}
					return nil
				}),
			huh.NewInput().Title("Expires").
				Description("72h, 2026-12-31, epoch — blank keeps it").
				Value(&v.expires),
			huh.NewSelect[string]().Title("Status").
				Options(huh.NewOptions("active", "inactive")...).
				Value(&v.status),
		),
	).WithShowHelp(true).WithTheme(huh.ThemeFunc(huh.ThemeCharm))

	e.open = true
	e.item = it
	e.vals = v
	e.form = form
	return e, e.form.Init()
}

// update advances the embedded form. done is true when the user
// submitted (StateCompleted); aborted is true on esc/ctrl+c.
func (e editForm) update(msg tea.Msg) (editForm, tea.Cmd, bool, bool) {
	model, cmd := e.form.Update(msg)
	if f, ok := model.(*huh.Form); ok {
		e.form = f
	}
	switch e.form.State {
	case huh.StateCompleted:
		e.open = false
		return e, cmd, true, false
	case huh.StateAborted:
		e.open = false
		return e, cmd, false, true
	}
	return e, cmd, false, false
}

func (e editForm) view() string { return e.form.View() }

// changes returns the PATCH body for fields that differ from the
// original link. Empty when nothing changed.
func (e editForm) changes() (map[string]any, error) {
	f := map[string]any{}
	if e.vals.longURL != e.item.LongURL {
		f["long_url"] = e.vals.longURL
	}
	if e.vals.alias != e.item.Alias {
		f["alias"] = e.vals.alias
	}
	if e.vals.password != "" {
		f["password"] = e.vals.password
	}
	if e.vals.maxClicks != "" {
		n, _ := strconv.Atoi(e.vals.maxClicks)
		cur := 0
		if e.item.MaxClicks != nil {
			cur = *e.item.MaxClicks
		}
		if n != cur {
			f["max_clicks"] = n
		}
	}
	if e.vals.expires != "" {
		exp, err := api.ParseExpiry(e.vals.expires, time.Now())
		if err != nil {
			return nil, err
		}
		f["expire_after"] = exp
	}
	if e.vals.status != strings.ToLower(e.item.Status) {
		f["status"] = e.vals.status
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
