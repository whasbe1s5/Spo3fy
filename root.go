package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/whasbe1s5/spo3fy-go/internal/config"
	"github.com/whasbe1s5/spo3fy-go/internal/downloader"
	"github.com/whasbe1s5/spo3fy-go/internal/scraper"
	"github.com/whasbe1s5/spo3fy-go/internal/tagger"
	"github.com/whasbe1s5/spo3fy-go/internal/tui"
	"github.com/whasbe1s5/spo3fy-go/internal/types"
)

var (
	flagType      string
	flagQuality   string
	flagFormat    string
	flagOutput    string
	flagGroup     string
	flagM3U       bool
	flagAllAlbums bool
	flagSkipCover bool
	flagVerbose   bool
)

var rootCmd = &cobra.Command{
	Use:   "spo3fy [URL]",
	Short: "Spotify downloader — no credentials needed",
	Long: `Spo3fy downloads Spotify tracks/albums/playlists to MP3 with full metadata and cover art.

No API credentials or premium subscription required.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return tui.Run()
		}

		return headlessDownload(args[0])
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

// qualityFromFlag maps the CLI quality string to a types.Quality.
func qualityFromFlag(s string) types.Quality {
	switch s {
	case "best":
		return types.QualityBest
	case "320k", "320":
		return types.Quality320k
	case "256k", "256":
		return types.Quality256k
	case "192k", "192":
		return types.Quality192k
	case "128k", "128":
		return types.Quality128k
	case "96k", "96":
		return types.Quality96k
	case "worst":
		return types.QualityWorst
	default:
		return types.QualityBest
	}
}

// formatFromFlag maps the CLI format string to a types.Format.
func formatFromFlag(s string) types.Format {
	switch s {
	case "mp3":
		return types.FormatMP3
	case "aac":
		return types.FormatAAC
	case "flac":
		return types.FormatFLAC
	case "m4a":
		return types.FormatM4A
	case "opus":
		return types.FormatOpus
	case "vorbis":
		return types.FormatVorbis
	case "wav":
		return types.FormatWAV
	default:
		return types.FormatMP3
	}
}

// headlessDownload runs a non-interactive download for the given URL or
// search query.  It resolves the URL to tracks, downloads each one, tags
// it, and optionally embeds cover art.
func headlessDownload(query string) error {
	quality := qualityFromFlag(flagQuality)
	format := formatFromFlag(flagFormat)

	fmt.Fprintf(os.Stderr, "Resolving: %s\n", query)

	tracks, err := scraper.Link(query)
	if err != nil {
		return fmt.Errorf("resolving URL: %w", err)
	}
	if len(tracks) == 0 {
		return fmt.Errorf("no tracks found for: %s", query)
	}
	fmt.Fprintf(os.Stderr, "Found %d track(s)\n", len(tracks))

	// Resolve output paths.
	paths := config.Resolve("", flagOutput)
	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Build grouping subdirectory when a group pattern is set.
	outBase := paths.OutDir
	if flagGroup != "" {
		outBase = filepath.Join(outBase, safeDirName(flagGroup))
	}

	m3uPaths := make([]string, 0, len(tracks))

	for i, track := range tracks {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", i+1, len(tracks), track)

		outputPath := filepath.Join(outBase, safeDirName(track.String()))
		searchQuery := fmt.Sprintf("ytsearch:%s audio", track)

		progress := make(chan downloader.ProgressUpdate, 64)
		done := make(chan error, 1)

		go func() {
			defer close(progress)
			done <- downloader.Download(searchQuery, quality, format, outputPath, progress, 3)
		}()
		// Show progress dots while downloading
		for range progress {
			fmt.Fprint(os.Stderr, ".")
		}

		if err := <-done; err != nil {
			fmt.Fprintf(os.Stderr, " ERROR: %v\n", err)
			continue
		}
		fmt.Fprintln(os.Stderr)

		audioPath := locateOutput(outputPath, string(format))
		if audioPath == "" {
			fmt.Fprintf(os.Stderr, "  output file not found for: %s\n", track)
			continue
		}

		// Tag metadata.
		if err := tagger.TagMetadata(audioPath, track, ""); err != nil {
			if flagVerbose {
				fmt.Fprintf(os.Stderr, "  tagging error: %v\n", err)
			}
		}

		// Embed cover art (unless skipped or default).
		if !flagSkipCover && !track.IsDefaultCover() && track.CoverArtURL != "" {
			if err := embedCoverTo(audioPath, track.CoverArtURL); err != nil {
				if flagVerbose {
					fmt.Fprintf(os.Stderr, "  cover art error: %v\n", err)
				}
			}
		}

		m3uPaths = append(m3uPaths, audioPath)
	}

	// Generate M3U playlist if requested.
	if flagM3U && len(m3uPaths) > 0 {
		if err := writeM3U(outBase, m3uPaths); err != nil {
			return fmt.Errorf("writing M3U: %w", err)
		}
	}

	return nil
}

// locateOutput finds the file produced by yt-dlp for the given outputPath and
// expected extension.  yt-dlp may write outputPath.ext or outputPath.%(ext)s,
// so we check both and fall back to globbing.
func locateOutput(basePath, expectedExt string) string {
	// Exact expected path.
	if _, err := os.Stat(basePath + "." + expectedExt); err == nil {
		return basePath + "." + expectedExt
	}
	// Globbing: find any file starting with basePath.
	matches, err := filepath.Glob(basePath + ".*")
	if err == nil && len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// embedCoverTo downloads cover art from coverURL and embeds it into audioPath
// via tagger.EmbedCoverArt.
func embedCoverTo(audioPath, coverURL string) error {
	coverDir, err := os.MkdirTemp("", "spo3fy-cover-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(coverDir)

	ext := ".jpg"
	if strings.HasSuffix(coverURL, ".png") {
		ext = ".png"
	}
	coverPath := filepath.Join(coverDir, "cover"+ext)

	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("too many redirects")
			}
			if req.URL.Scheme != "http" && req.URL.Scheme != "https" {
				return fmt.Errorf("redirect to non-HTTP(S) scheme: %s", req.URL.Scheme)
			}
			return nil
		},
	}
	resp, err := client.Get(coverURL)
	if err != nil {
		return fmt.Errorf("downloading cover art: %w", err)
	}
	defer resp.Body.Close()

	f, err := os.Create(coverPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return fmt.Errorf("writing cover art: %w", err)
	}
	f.Close()

	return tagger.EmbedCoverArt(audioPath, coverPath, "")
}

// safeDirName replaces filesystem-unsafe characters with underscores.
func safeDirName(name string) string {
	r := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return r.Replace(name)
}

// writeM3U writes an extended M3U playlist file listing the given audio paths
// relative to the playlist directory.
func writeM3U(dir string, paths []string) error {
	m3uPath := filepath.Join(dir, "playlist.m3u")
	f, err := os.Create(m3uPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	fmt.Fprintln(w, "#EXTM3U")
	for _, p := range paths {
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			rel = p
		}
		fmt.Fprintln(w, rel)
	}
	return w.Flush()
}

func init() {
	rootCmd.Flags().StringVarP(&flagType, "type", "t", "track",
		"Resource type: track|album|playlist|artist")
	rootCmd.Flags().StringVarP(&flagQuality, "quality", "q", "best",
		"Audio quality: best|320k|256k|192k|128k|96k|worst")
	rootCmd.Flags().StringVarP(&flagFormat, "format", "f", "mp3",
		"Output format: mp3|aac|flac|m4a|opus|vorbis|wav")
	rootCmd.Flags().StringVarP(&flagOutput, "output", "o", "",
		"Output directory (default: ~/Music/Spo3fy)")
	rootCmd.Flags().StringVarP(&flagGroup, "group", "g", "",
		"Grouping pattern (subdirectory name for output)")
	rootCmd.Flags().BoolVarP(&flagM3U, "m3u", "m", false,
		"Create M3U playlist file")
	rootCmd.Flags().BoolVarP(&flagAllAlbums, "all-albums", "a", false,
		"Download all artist albums")
	rootCmd.Flags().BoolVar(&flagSkipCover, "skip-cover-art", false,
		"Skip cover art embedding")
	rootCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false,
		"Verbose logging")
}
