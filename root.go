package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/whasbe1s5/Spo3fy/internal/config"
	"github.com/whasbe1s5/Spo3fy/internal/downloader"
	"github.com/whasbe1s5/Spo3fy/internal/scraper"
	"github.com/whasbe1s5/Spo3fy/internal/tagger"
	"github.com/whasbe1s5/Spo3fy/internal/types"
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
			return cmd.Help()
		}
		return headlessDownload(args[0])
	},
	SilenceUsage:  true,
	SilenceErrors: true,
}

func qualityFromFlag(s string) types.Quality {
	switch s {
	case "best":
		return types.QualityBest
	case "320k":
		return types.Quality320k
	case "256k":
		return types.Quality256k
	case "192k":
		return types.Quality192k
	case "128k":
		return types.Quality128k
	case "96k":
		return types.Quality96k
	case "worst":
		return types.QualityWorst
	default:
		return types.QualityBest
	}
}

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

// headlessDownload downloads tracks from a Spotify URL.
// Albums and playlists get their own subfolder with the container cover image.
func headlessDownload(query string) error {
	quality := qualityFromFlag(flagQuality)
	format := formatFromFlag(flagFormat)

	fmt.Fprintf(os.Stderr, "Resolving: %s\n", query)

	result, err := scraper.LinkWithMeta(query)
	if err != nil {
		return fmt.Errorf("resolving URL: %w", err)
	}
	if len(result.Tracks) == 0 {
		return fmt.Errorf("no tracks found for: %s", query)
	}
	fmt.Fprintf(os.Stderr, "Found %d track(s)\n", len(result.Tracks))

	paths := config.Resolve("", flagOutput)
	if err := paths.EnsureDirs(); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	// Albums and playlists get a subfolder named after the container.
	outBase := paths.OutDir
	if result.Name != "" {
		if flagGroup != "" {
			outBase = filepath.Join(outBase, config.SafeFilename(flagGroup))
		} else {
			outBase = filepath.Join(outBase, config.SafeFilename(result.Name))
		}
	} else if flagGroup != "" {
		outBase = filepath.Join(outBase, config.SafeFilename(flagGroup))
	}
	if err := os.MkdirAll(outBase, 0o755); err != nil {
		return fmt.Errorf("creating output directory %s: %w", outBase, err)
	}

	// Ensure ffmpeg.
	ffmpegPath := ""
	if tagger.IsAvailable("") {
		ffmpegPath = "ffmpeg"
	} else if p, ok := tagger.IsDownloaded(paths.DataDir); ok {
		ffmpegPath = p
	} else {
		fmt.Fprintln(os.Stderr, "ffmpeg not found. Downloading...")
		p, err := tagger.DownloadFFmpeg(paths.DataDir, nil)
		if err != nil {
			return fmt.Errorf("failed to download ffmpeg: %w", err)
		}
		ffmpegPath = p
	}

	// Download album/playlist cover image into the folder.
	if result.ImageURL != "" && result.ImageURL != types.DefaultCoverArtURL {
		if err := downloadFile(result.ImageURL, filepath.Join(outBase, "cover.jpg")); err != nil {
			if flagVerbose {
				fmt.Fprintf(os.Stderr, "  container cover error: %v\n", err)
			}
		}
	}

	m3uPaths := make([]string, 0, len(result.Tracks))

	for i, track := range result.Tracks {
		fmt.Fprintf(os.Stderr, "[%d/%d] %s\n", i+1, len(result.Tracks), track)

		outputPath := filepath.Join(outBase, config.SafeFilename(track.String()))
		searchQuery := fmt.Sprintf("ytsearch:%s audio", track)

		progress := make(chan downloader.ProgressUpdate, 64)
		done := make(chan error, 1)

		go func() {
			defer close(progress)
			done <- downloader.Download(searchQuery, quality, format, outputPath, progress, 3)
		}()
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

		if err := tagger.TagMetadata(audioPath, track, ffmpegPath); err != nil {
			if flagVerbose {
				fmt.Fprintf(os.Stderr, "  tagging error: %v\n", err)
			}
		}

		if !flagSkipCover && !track.IsDefaultCover() && track.CoverArtURL != "" {
			if err := tagger.DownloadAndEmbedCover(audioPath, track.CoverArtURL, ffmpegPath); err != nil {
				if flagVerbose {
					fmt.Fprintf(os.Stderr, "  cover art error: %v\n", err)
				}
			}
		}

		m3uPaths = append(m3uPaths, audioPath)
	}

	if flagM3U && len(m3uPaths) > 0 {
		if err := writeM3U(outBase, m3uPaths); err != nil {
			return fmt.Errorf("writing M3U: %w", err)
		}
	}

	return nil
}

func locateOutput(basePath, expectedExt string) string {
	if _, err := os.Stat(basePath + "." + expectedExt); err == nil {
		return basePath + "." + expectedExt
	}
	matches, _ := filepath.Glob(basePath + ".*")
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

func downloadFile(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func writeM3U(dir string, paths []string) error {
	f, err := os.Create(filepath.Join(dir, "playlist.m3u"))
	if err != nil {
		return err
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	w.WriteString("#EXTM3U\n")
	for _, p := range paths {
		w.WriteString(filepath.Base(p) + "\n")
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
		"Override subfolder name for album/playlist (or name it for single tracks)")
	rootCmd.Flags().BoolVarP(&flagM3U, "m3u", "m", false,
		"Create M3U playlist file")
	rootCmd.Flags().BoolVarP(&flagAllAlbums, "all-albums", "a", false,
		"Download all artist albums")
	rootCmd.Flags().BoolVar(&flagSkipCover, "skip-cover-art", false,
		"Skip cover art embedding")
	rootCmd.Flags().BoolVarP(&flagVerbose, "verbose", "v", false,
		"Verbose logging")
}
