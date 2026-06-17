// Package downloader wraps yt-dlp for audio downloading and ffmpeg extraction.
package downloader

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/whasbe1s/spo3fy-go/internal/types"
)

// ProgressUpdate holds the current download progress for the TUI.
type ProgressUpdate struct {
	Percent   float64
	TrackName string
	Status    string // "downloading", "processing", "done", "error"
}

// Download downloads a track via yt-dlp + ffmpeg extraction.
// The yt-dlp binary must be on PATH.
// query: "ytsearch:<Artist - Track Name> audio"
// quality: yt-dlp format string (0=best, 320=320kbps, etc.)
// format: output extension (mp3, flac, etc.)
// outputPath: full output file path (WITHOUT extension)
// Progress is sent to the progress channel.
// Returns nil on success, error on failure (after retries).
func Download(
	query string,
	quality types.Quality,
	format types.Format,
	outputPath string,
	progress chan<- ProgressUpdate,
	retries int,
) error {
	if retries < 1 {
		retries = 3
	}

	trackName := extractTrackName(query)

	// Build yt-dlp arguments per contract specification.
	args := []string{
		"--format", "bestaudio/best",
		"--output", outputPath + ".%(ext)s",
		"--restrict-filenames",
		"--ignore-errors",
		"--no-overwrites",
		"--no-playlist",
		"--prefer-ffmpeg",
		"--extract-audio",
		"--audio-format", string(format),
		"--audio-quality", string(quality),
		"--postprocessor-args", postProcessorArgs(trackName),
		query,
	}

	progress <- ProgressUpdate{
		Percent:   0,
		TrackName: trackName,
		Status:    "downloading",
	}

	re := regexp.MustCompile(`\[download\]\s+(\d+\.?\d*)%`)

	var lastErr error
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			progress <- ProgressUpdate{
				Percent:   0,
				TrackName: trackName,
				Status:    fmt.Sprintf("downloading (retry %d/%d)", attempt+1, retries),
			}
		}

		cmd := exec.Command("yt-dlp", args...)

		stdout, err := cmd.StdoutPipe()
		if err != nil {
			lastErr = fmt.Errorf("stdout pipe: %w", err)
			continue
		}

		if err := cmd.Start(); err != nil {
			lastErr = fmt.Errorf("start yt-dlp: %w", err)
			continue
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			matches := re.FindStringSubmatch(line)
			if matches == nil {
				continue
			}
			pct, err := strconv.ParseFloat(matches[1], 64)
			if err != nil {
				continue
			}
			progress <- ProgressUpdate{
				Percent:   pct,
				TrackName: trackName,
				Status:    "downloading",
			}
		}

		waitErr := cmd.Wait()
		if waitErr == nil {
			progress <- ProgressUpdate{
				Percent:   100,
				TrackName: trackName,
				Status:    "processing",
			}
			return nil
		}

		lastErr = fmt.Errorf("attempt %d/%d: %w", attempt+1, retries, waitErr)
		if scErr := scanner.Err(); scErr != nil {
			lastErr = fmt.Errorf("%v; stdout scanner: %w", lastErr, scErr)
		}
	}

	progress <- ProgressUpdate{
		Percent:   0,
		TrackName: trackName,
		Status:    "error",
	}
	return fmt.Errorf("download failed after %d attempts: %w", retries, lastErr)
}

// extractTrackName parses "ytsearch:Artist - Track Name audio" into
// "Artist - Track Name" for display in progress updates.
func extractTrackName(query string) string {
	const prefix = "ytsearch:"
	if !strings.HasPrefix(query, prefix) {
		return query
	}
	rest := query[len(prefix):]
	// Trim trailing " audio" suffix from the ytsearch query.
	if idx := strings.LastIndex(rest, " audio"); idx >= 0 {
		rest = rest[:idx]
	}
	return rest
}

// postProcessorArgs builds the --postprocessor-args value for yt-dlp's
// ffmpeg post-processor, injecting track metadata into the output file.
func postProcessorArgs(trackName string) string {
	artist := ""
	title := trackName
	if idx := strings.Index(trackName, " - "); idx >= 0 {
		artist = trackName[:idx]
		title = trackName[idx+3:]
	}

	// Fields we cannot derive (album, date, disc, track number) are left empty;
	// ffmpeg simply skips those tags when the value is empty.
	return fmt.Sprintf(
		"-write_id3v1 1 -id3v2_version 3 -metadata title=%s -metadata album= -metadata date= -metadata artist=%s -metadata disc=1 -metadata track=",
		ffmpegQuote(title),
		ffmpegQuote(artist),
	)
}

// ffmpegQuote wraps a value in quotes if it contains spaces, to satisfy
// yt-dlp's argument splitting for the --postprocessor-args flag.
func ffmpegQuote(s string) string {
	if s == "" {
		return ""
	}
	if strings.Contains(s, " ") {
		return `"` + s + `"`
	}
	return s
}
