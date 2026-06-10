// Package ui holds the lipgloss styles shared by every spoo command,
// so CLI output and future TUI views render with one visual language.
package ui

import lipgloss "charm.land/lipgloss/v2"

var (
	Accent  = lipgloss.Color("#A78BFA") // spoo violet
	Success = lipgloss.Color("#34D399")
	Danger  = lipgloss.Color("#F87171")
	Muted   = lipgloss.Color("#9CA3AF")

	Title   = lipgloss.NewStyle().Bold(true).Foreground(Accent)
	OK      = lipgloss.NewStyle().Bold(true).Foreground(Success)
	Err     = lipgloss.NewStyle().Bold(true).Foreground(Danger)
	Dim     = lipgloss.NewStyle().Foreground(Muted)
	Box     = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(Accent).Padding(0, 2)
	KeyHint = lipgloss.NewStyle().Foreground(Muted).Italic(true)
)

// SparkRunes are the eight block heights used for sparkline charts.
var SparkRunes = []rune("▁▂▃▄▅▆▇█")
