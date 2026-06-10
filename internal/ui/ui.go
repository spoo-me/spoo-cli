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

// CountryLabel renders an ISO alpha-2 country code with its flag emoji
// (regional indicator pairs). The backend reports unknown geo as "XX",
// which has no flag.
func CountryLabel(code string) string {
	if code == "XX" {
		return "Unknown" // match the backend's casing for unknown cities
	}
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return code
	}
	flag := string(rune(0x1F1E6+int(code[0]-'A'))) + string(rune(0x1F1E6+int(code[1]-'A')))
	return flag + " " + code
}
