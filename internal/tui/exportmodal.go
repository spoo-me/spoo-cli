package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/spoo-me/spoo-cli/internal/ui"
)

// exportFormats maps filename extensions to the API's export formats.
// csv arrives as a zip archive, so .csv is normalized to .zip.
var exportFormats = map[string]string{
	"xlsx": "xlsx",
	"json": "json",
	"xml":  "xml",
	"csv":  "csv",
	"zip":  "csv",
}

// exportModal is the centered export dialog shared by the stats and
// links TUIs: a filename field (extension picks the format) over a
// directory browser. tab switches panes, enter confirms, esc closes.
type exportModal struct {
	open   bool
	name   textinput.Model
	picker filepicker.Model
	pane   int // 0 = filename field, 1 = directory browser
	dir    string
	err    string
}

// exportRequest is what a confirmed modal hands back to its host.
type exportRequest struct {
	path   string // absolute destination
	format string // API format derived from the extension
}

func newExportModal() exportModal {
	name := textinput.New()
	name.Placeholder = "filename…"
	name.SetWidth(44)
	picker := filepicker.New()
	picker.DirAllowed = true
	picker.FileAllowed = false
	picker.AutoHeight = false
	picker.SetHeight(8)
	return exportModal{name: name, picker: picker}
}

// show opens the modal with a suggested filename, browsing from the
// working directory.
func (e exportModal) show(defaultName string) (exportModal, tea.Cmd) {
	e.open = true
	e.err = ""
	e.pane = 0
	e.name.SetValue(defaultName)
	if wd, err := os.Getwd(); err == nil {
		e.dir = wd
		e.picker.CurrentDirectory = wd
	}
	return e, tea.Batch(e.picker.Init(), e.name.Focus())
}

// confirm validates the filename and resolves the request.
func (e exportModal) confirm() (exportModal, *exportRequest) {
	name := strings.TrimSpace(e.name.Value())
	if name == "" {
		e.err = "name the file"
		return e, nil
	}
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	format, ok := exportFormats[ext]
	if !ok {
		e.err = "use .xlsx, .json, .xml, or .zip (csv)"
		return e, nil
	}
	if ext == "csv" { // csv ships zipped — keep the name honest
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".zip"
	}
	e.open = false
	return e, &exportRequest{path: filepath.Join(e.dir, name), format: format}
}

// handle routes a message through the modal. A non-nil request means
// the user confirmed an export; the host owns actually running it.
func (e exportModal) handle(msg tea.Msg) (exportModal, *exportRequest, tea.Cmd) {
	if key, ok := msg.(tea.KeyPressMsg); ok {
		switch key.String() {
		case "ctrl+c":
			return e, nil, tea.Quit
		case "esc":
			e.open = false
			return e, nil, nil
		case "tab", "shift+tab":
			e.pane = 1 - e.pane
			if e.pane == 0 {
				return e, nil, e.name.Focus()
			}
			e.name.Blur()
			return e, nil, nil
		case "enter":
			if e.pane == 0 {
				next, req := e.confirm()
				return next, req, nil
			}
		}
		var cmd tea.Cmd
		if e.pane == 0 {
			e.name, cmd = e.name.Update(msg)
			e.err = ""
			return e, nil, cmd
		}
		e.picker, cmd = e.picker.Update(msg)
		if ok, path := e.picker.DidSelectFile(msg); ok {
			e.dir = path
			e.pane = 0
			return e, nil, tea.Batch(cmd, e.name.Focus())
		}
		return e, nil, cmd
	}

	// non-key traffic (directory reads, cursor blinks) goes to both
	var nameCmd, pickerCmd tea.Cmd
	e.name, nameCmd = e.name.Update(msg)
	e.picker, pickerCmd = e.picker.Update(msg)
	return e, nil, tea.Batch(nameCmd, pickerCmd)
}

// view renders the modal centered in the given screen box.
func (e exportModal) view(width, height int) string {
	label := func(s string, active bool) string {
		if active {
			return ui.Title.Render(s)
		}
		return ui.Dim.Render(s)
	}
	lines := []string{
		ui.Title.Render("✦ export analytics"),
		"",
		label("file ", e.pane == 0) + " " + e.name.View(),
		label("into ", e.pane == 1) + " " + ui.Dim.Render(collapseHome(e.dir)),
		"",
	}
	if e.pane == 1 {
		lines = append(lines, e.picker.View())
	}
	switch {
	case e.err != "":
		lines = append(lines, ui.Err.Render("✗ "+e.err))
	case e.pane == 1:
		lines = append(lines, ui.KeyHint.Render("↑/↓ browse · enter choose dir · tab back to name · esc cancel"))
	default:
		lines = append(lines, ui.KeyHint.Render("extension picks the format · tab choose folder · enter export · esc cancel"))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ui.Accent).
		Padding(0, 2).
		Width(min(64, max(40, width-8))).
		Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}

// collapseHome shortens a path for display (~/… instead of /Users/…).
func collapseHome(path string) string {
	if home, err := os.UserHomeDir(); err == nil {
		if rest, ok := strings.CutPrefix(path, home); ok {
			return "~" + rest
		}
	}
	return path
}

// defaultExportName builds the suggested filename for an export.
func defaultExportName(subject, date string) string {
	return fmt.Sprintf("spoo-%s-%s.xlsx", subject, date)
}
