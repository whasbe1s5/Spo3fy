// Package tagger wraps FFmpeg for audio post-processing: metadata tagging
// and cover-art embedding. These functions expect ffmpeg to be available
// either on PATH or at an explicit path.
package tagger

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/whasbe1s5/spo3fy-go/internal/types"
)

// IsAvailable reports whether ffmpeg is found on PATH (when ffmpegPath is
// empty) or at the given explicit path.  It does not verify that the binary
// actually works.
func IsAvailable(ffmpegPath string) bool {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	_, err := exec.LookPath(ffmpegPath)
	return err == nil
}

// EmbedCoverArt runs ffmpeg to embed a cover image into an MP3 file. Because
// ffmpeg cannot read and write the same file, the function writes to a
// temporary file next to audioPath and then renames it atomically.
//
// The ffmpeg invocation is:
//
//	ffmpeg -i <audio> -i <cover> -map 0:0 -map 1:0 -c copy
//	    -id3v2_version 3
//	    -metadata:s:v title="Album cover"
//	    -metadata:s:v comment="Cover (front)"
//	    <temp> -loglevel quiet -hide_banner -y
func EmbedCoverArt(audioPath, coverArtPath, ffmpegPath string) error {
	binary := ffmpegPath
	if binary == "" {
		binary = "ffmpeg"
	}

	tmpPath := audioPath + ".tmp"

	args := []string{
		"-i", audioPath,
		"-i", coverArtPath,
		"-map", "0:0",
		"-map", "1:0",
		"-c", "copy",
		"-id3v2_version", "3",
		"-metadata:s:v", "title=Album cover",
		"-metadata:s:v", "comment=Cover (front)",
		tmpPath,
		"-loglevel", "quiet",
		"-hide_banner",
		"-y",
	}

	cmd := exec.Command(binary, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("embed cover art: %w\n%s", err, string(out))
	}

	return os.Rename(tmpPath, audioPath)
}

// TagMetadata runs ffmpeg to write ID3 metadata from the Track struct into
// the audio file.  Like EmbedCoverArt, it uses a temporary output to avoid
// the read/write-same-file problem and renames afterwards.
//
// Track fields are mapped to ID3 tags as follows:
//   - Name          → title
//   - Artists       → artist  (joined with " / ")
//   - AlbumName     → album
//   - ReleaseDate   → date
//   - TrackNumber   → track   ("N/M" when AlbumTrackCount > 0)
//   - DiscNumber    → disc
//
// The audio stream is copied without re-encoding (-codec copy).
func TagMetadata(audioPath string, track types.Track, ffmpegPath string) error {
	binary := ffmpegPath
	if binary == "" {
		binary = "ffmpeg"
	}

	artist := strings.Join(track.Artists, " / ")

	trackTag := strconv.Itoa(track.TrackNumber)
	if track.AlbumTrackCount > 0 {
		trackTag += "/" + strconv.Itoa(track.AlbumTrackCount)
	}

	tmpPath := audioPath + ".tmp"

	args := []string{
		"-i", audioPath,
		"-metadata", fmt.Sprintf("title=%s", track.Name),
		"-metadata", fmt.Sprintf("artist=%s", artist),
		"-metadata", fmt.Sprintf("album=%s", track.AlbumName),
		"-metadata", fmt.Sprintf("date=%s", track.ReleaseDate),
		"-metadata", fmt.Sprintf("track=%s", trackTag),
		"-metadata", fmt.Sprintf("disc=%d", track.DiscNumber),
		"-codec", "copy",
		tmpPath,
		"-loglevel", "quiet",
		"-hide_banner",
		"-y",
	}

	cmd := exec.Command(binary, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("tag metadata: %w\n%s", err, string(out))
	}

	return os.Rename(tmpPath, audioPath)
}
