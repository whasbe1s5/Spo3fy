package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ── Settings screen ─────────────────────────────────────────────────────

// settingsView renders the settings panel, URL input, and key bindings.
func settingsView(m model) string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("                  _____  __       \n" +
		"                 |____ |/ _|      \n" +
		" ___ _ __   ___      / / |_ _   _ \n" +
		`/ __| '_ \ / _ \     \ \  _| | | |` + "\n" +
		`\__ \ |_) | (_) |.___/ / | | |_| |` + "\n" +
		`|___/ .__/ \___/ \____/|_|  \__, |` + "\n" +
		"    | |                      __/ |\n" +
		"    |_|                     |___/ \n"))
	b.WriteString("\n")

	// ── Settings panel ────────────────────────────────────────────────
	if m.editingField > 0 {
		b.WriteString(renderEditingPrompt(m))
	} else {
		b.WriteString(renderSettingsTable(m))
	}

	b.WriteString("\n\n")

	// ── URL input ─────────────────────────────────────────────────────
	prompt := InputPromptStyle.Render("> ")
	if m.editingField > 0 {
		b.WriteString(SettingsKeyStyle.Render("Value:"))
		b.WriteString(" " + InputPromptStyle.Render(m.editBuffer))
		if m.cursorVisible {
			b.WriteString("▎")
		}
		if m.editBuffer == "" {
			b.WriteString(HelpStyle.Render(" (type value, Enter to confirm, Esc to cancel)"))
		}
	} else {
		b.WriteString(prompt)
		b.WriteString(m.urlInput)
		if m.cursorVisible {
			b.WriteString("▎")
		}
		if m.urlInput == "" {
			b.WriteString(HelpStyle.Render(" (paste a Spotify URL)"))
		}
	}
	b.WriteString("\n\n")

	// ── Help text ─────────────────────────────────────────────────────
	b.WriteString(HelpStyle.Render("q / Ctrl+C: quit  |  Enter: start download  |  1-7: change setting"))
	b.WriteString("\n")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		PanelStyle.Width(clampWidth(m.width)).Render(b.String()))
}

func renderSettingsTable(m model) string {
	var sb strings.Builder
	sb.WriteString(SettingsKeyStyle.Render(fmt.Sprintf("  Quality    ")) +
		SettingsValStyle.Render(string(m.cfg.quality)))
	sb.WriteString(HelpStyle.Render("  [1]"))
	sb.WriteString("\n")
	sb.WriteString(HelpStyle.Render("      Higher bitrate = larger file, better sound"))
	sb.WriteString("\n")

	sb.WriteString(SettingsKeyStyle.Render(fmt.Sprintf("  Format     ")) +
		SettingsValStyle.Render(string(m.cfg.format)))
	sb.WriteString(HelpStyle.Render("  [2]"))
	sb.WriteString("\n")
	sb.WriteString(HelpStyle.Render("      Output audio container format"))
	sb.WriteString("\n")

	sb.WriteString(SettingsKeyStyle.Render(fmt.Sprintf("  Output Dir ")) +
		SettingsValStyle.Render(m.cfg.outputDir))
	sb.WriteString(HelpStyle.Render("  [3]"))
	sb.WriteString("\n")
	sb.WriteString(HelpStyle.Render("      Where downloaded files are saved"))
	sb.WriteString("\n")

	sb.WriteString(SettingsKeyStyle.Render(fmt.Sprintf("  Group Dir  ")) +
		SettingsValStyle.Render(boolStr(m.cfg.groupDir)))
	sb.WriteString(HelpStyle.Render("  [4]"))
	sb.WriteString("\n")
	sb.WriteString(HelpStyle.Render("      Put tracks in album/playlist subfolders"))
	sb.WriteString("\n")

	sb.WriteString(SettingsKeyStyle.Render(fmt.Sprintf("  Create M3U ")) +
		SettingsValStyle.Render(boolStr(m.cfg.createM3U)))
	sb.WriteString(HelpStyle.Render("  [5]"))
	sb.WriteString("\n")
	sb.WriteString(HelpStyle.Render("      Generate an .m3u playlist file"))
	sb.WriteString("\n")

	sb.WriteString(SettingsKeyStyle.Render(fmt.Sprintf("  Skip Cover ")) +
		SettingsValStyle.Render(boolStr(m.cfg.skipCoverArt)))
	sb.WriteString(HelpStyle.Render("  [6]"))
	sb.WriteString("\n")
	sb.WriteString(HelpStyle.Render("      Don't embed cover art in audio files"))
	sb.WriteString("\n")

	sb.WriteString(SettingsKeyStyle.Render(fmt.Sprintf("  All Albums ")) +
		SettingsValStyle.Render(boolStr(m.cfg.allAlbums)))
	sb.WriteString(HelpStyle.Render("  [7]"))
	sb.WriteString("\n")
	sb.WriteString(HelpStyle.Render("      Download every album by an artist"))

	return sb.String()
}

func renderEditingPrompt(m model) string {
	result := fmt.Sprintf("%s\n%s",
		SettingsKeyStyle.Render("Editing Output Directory"),
		InputPromptStyle.Render("  "+m.editBuffer))
	if m.editBuffer == "" {
		result += "\n" + HelpStyle.Render("  enter a folder path")
	}
	result += "\n" + HelpStyle.Render("  Enter to confirm, Esc to cancel")
	return result
}

// ── Progress screen ─────────────────────────────────────────────────────

// progressView renders the live download progress view.
func progressView(m model) string {
	var b strings.Builder
	total := m.downloadsTotal
	if total == 0 {
		total = 1
	}

	// Overall progress: completed tracks + current track's fraction
	overall := (float64(m.downloadsDone) + m.progress.Percent/100.0) / float64(total) * 100.0

	b.WriteString(TitleStyle.Render("⬇ Spo3fy — Downloading"))
	b.WriteString("\n\n")

	// Spinner + track count
	sp := SpinnerStyle.Render(m.spinner.View())
	b.WriteString(sp)
	b.WriteString(" ")
	b.WriteString(fmt.Sprintf("[%d/%d]", m.downloadsDone+1, m.downloadsTotal))
	b.WriteString("\n\n")

	// Current track name
	if m.progress.TrackName != "" {
		b.WriteString(m.progress.TrackName)
		b.WriteString("\n")
	}

	// Status label
	status := m.progress.Status
	if status == "" {
		status = "starting…"
	}
	b.WriteString(HelpStyle.Render(status))
	b.WriteString("\n")

	// Overall progress bar
	barWidth := clampWidth(m.width) - 4
	if barWidth < 10 {
		barWidth = 10
	}
	bar := ProgressBar(overall, barWidth)
	b.WriteString("  ")
	b.WriteString(bar)
	b.WriteString(fmt.Sprintf("  %.0f%%", overall))
	b.WriteString("\n\n")

	b.WriteString(HelpStyle.Render("q / Ctrl+C: cancel and quit"))
	b.WriteString("\n")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		PanelStyle.Width(clampWidth(m.width)).Render(b.String()))
}

// ── Results screen ──────────────────────────────────────────────────────

// resultsView renders the download summary after all tracks finish.
func resultsView(m model) string {
	var b strings.Builder

	b.WriteString(TitleStyle.Render("✓ Spo3fy — Complete"))
	b.WriteString("\n\n")

	successCount := 0
	failCount := 0
	var failedTracks []string

	for _, r := range m.results {
		if r.Success {
			successCount++
		} else {
			failCount++
			failedTracks = append(failedTracks,
				fmt.Sprintf("  ✗ %s — %s", r.Track.String(), r.Error))
		}
	}

	// Summary line
	summary := fmt.Sprintf("  %s %d downloaded", SuccessStyle.Render("✔"), successCount)
	if failCount > 0 {
		summary += fmt.Sprintf("  |  %s %d failed", ErrorStyle.Render("✘"), failCount)
	}
	b.WriteString(summary)
	b.WriteString("\n\n")

	// Failed tracks detail
	if failCount > 0 {
		b.WriteString(ErrorStyle.Render("Failures:"))
		b.WriteString("\n")
		for _, f := range failedTracks {
			b.WriteString(f)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Prompt to continue
	b.WriteString(HelpStyle.Render("Press Enter to return to settings, q / Ctrl+C to quit"))
	b.WriteString("\n")

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
		PanelStyle.Width(clampWidth(m.width)).Render(b.String()))
}

// ── Helpers ─────────────────────────────────────────────────────────────

func boolStr(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func clampWidth(w int) int {
	if w > 120 {
		return 120
	}
	if w < 40 {
		return 40
	}
	return w
}
