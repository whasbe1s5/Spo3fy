// Package tui implements a Bubble Tea terminal UI for Spo3fy.
package tui

import (
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Colour constants ────────────────────────────────────────────────────
const (
	colCyan   = lipgloss.Color("14")
	colYellow = lipgloss.Color("11")
	colRed    = lipgloss.Color("9")
	colGreen  = lipgloss.Color("10")
	colBlue   = lipgloss.Color("12")
	colPink   = lipgloss.Color("205")
)

// ── Exported styles ─────────────────────────────────────────────────────
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(colCyan).
			Align(lipgloss.Center).
			Margin(1, 0)

	InputPromptStyle = lipgloss.NewStyle().
				Foreground(colCyan)

	SettingsKeyStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colCyan)

	SettingsValStyle = lipgloss.NewStyle().
				Foreground(colYellow)

	SpinnerStyle = lipgloss.NewStyle().
			Foreground(colPink)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(colRed)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(colGreen)

	HelpStyle = lipgloss.NewStyle().
			Faint(true)

	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colBlue).
			Padding(1, 2)

	// Internal progress bar sub-styles — use ProgressBar() to render.
	progressBarFilled = lipgloss.NewStyle().Foreground(colCyan)
	progressBarEmpty  = lipgloss.NewStyle().Faint(true)
)

// ProgressBar renders a visual progress bar of the given width.
// percent is expected in 0.0–100.0 range.
func ProgressBar(percent float64, width int) string {
	clamped := math.Max(0, math.Min(100, percent))
	filled := int(math.Round(clamped * float64(width) / 100))
	if filled < 0 {
		filled = 0
	}
	if filled > width {
		filled = width
	}
	empty := width - filled

	filledStr := strings.Repeat("━", filled)
	emptyStr := strings.Repeat("─", empty)

	return progressBarFilled.Render(filledStr) +
		progressBarEmpty.Render(emptyStr)
}
