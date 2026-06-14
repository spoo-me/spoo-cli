package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// confirmDialog is the shared confirmation modal. Simple mode confirms
// on enter; phrase mode (used for destructive actions) requires the
// user to type an exact phrase first. The host reads tag to know which
// action to run when confirmed.
type confirmDialog struct {
	open   bool
	tag    string   // host-defined: which pending action this confirms
	tagID  string   // host-defined: the id the action targets
	title  string   // headline
	lines  []string // body context
	phrase string   // type-to-confirm target; empty = simple confirm
	danger bool     // red accent for destructive actions
	input  textinput.Model
}

func newConfirmDialog() confirmDialog {
	in := textinput.New()
	in.SetWidth(36)
	return confirmDialog{input: in}
}

// askSimple opens an enter-to-confirm dialog for action tag on id.
func (c confirmDialog) askSimple(tag, id, title string, lines []string) confirmDialog {
	c.open, c.tag, c.tagID, c.title, c.lines = true, tag, id, title, lines
	c.phrase, c.danger = "", false
	c.input.SetValue("")
	c.input.Blur()
	return c
}

// askPhrase opens a type-the-phrase-to-confirm dialog (destructive).
func (c confirmDialog) askPhrase(tag, id, title string, lines []string, phrase string) confirmDialog {
	c.open, c.tag, c.tagID, c.title, c.lines = true, tag, id, title, lines
	c.phrase, c.danger = phrase, true
	c.input.SetValue("")
	c.input.Placeholder = phrase
	c.input.Focus()
	return c
}

// handle routes a key. confirmed is true only when the user committed
// (and, in phrase mode, typed the phrase exactly). When the dialog
// resolves either way it closes itself.
func (c confirmDialog) handle(msg tea.KeyPressMsg) (confirmDialog, bool, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return c, false, tea.Quit
	case "esc":
		c.open = false
		return c, false, nil
	case "enter":
		if c.phrase == "" {
			c.open = false
			return c, true, nil
		}
		if c.input.Value() == c.phrase {
			c.open = false
			return c, true, nil
		}
		return c, false, nil // mismatch: keep waiting
	}
	if c.phrase != "" {
		var cmd tea.Cmd
		c.input, cmd = c.input.Update(msg)
		return c, false, cmd
	}
	return c, false, nil
}

func (c confirmDialog) view() string {
	accent := ui.Accent
	titleStyle := ui.Title
	if c.danger {
		accent = ui.Danger
		titleStyle = ui.Err
	}

	out := []string{titleStyle.Render(c.title), ""}
	out = append(out, c.lines...)

	if c.phrase != "" {
		matched := c.input.Value() == c.phrase
		hint := ui.Dim.Render("type ") + ui.Title.Render(c.phrase) + ui.Dim.Render(" to confirm")
		out = append(out, "", hint, c.input.View())
		action := ui.Dim.Render("enter delete")
		if !matched {
			action = lipgloss.NewStyle().Foreground(ui.Muted).Render("enter delete")
		} else {
			action = ui.Err.Render("enter delete")
		}
		out = append(out, "", action+ui.Dim.Render(" · esc cancel"))
	} else {
		out = append(out, "", ui.OK.Render("enter confirm")+ui.Dim.Render(" · esc cancel"))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 2).
		Width(min(60, max(44, lipgloss.Width(strings.Join(out, "\n"))+6))).
		Render(strings.Join(out, "\n"))
}
