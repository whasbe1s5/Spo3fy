// Package tagger handles audio tagging and FFmpeg lifecycle.
package tagger

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// IsDownloaded checks if ffmpeg was previously downloaded to dataDir.
// Returns the path to the binary and true if it exists and is executable.
func IsDownloaded(dataDir string) (string, bool) {
	path := filepath.Join(dataDir, "ffmpeg", "ffmpeg")
	info, err := os.Stat(path)
	if err != nil {
		return "", false
	}
	if info.Mode().Perm()&0o111 == 0 {
		return "", false
	}
	return path, true
}

// DownloadFFmpeg downloads the static FFmpeg binary for the current OS/arch.
// Supported: darwin/arm64 (Apple Silicon), darwin/amd64 (Intel).
// Returns the path to the downloaded binary, stored in <dataDir>/ffmpeg/ffmpeg.
func DownloadFFmpeg(dataDir string, progress chan<- string) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", errors.New("unsupported OS")
	}

	var zipURL string
	switch runtime.GOARCH {
	case "arm64":
		zipURL = "https://evermeet.cx/ffmpeg/get/zip"
	case "amd64":
		zipURL = "https://evermeet.cx/ffmpeg/getrelease/zip"
	default:
		return "", errors.New("unsupported OS")
	}

	sendProgress(progress, "downloading...")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(zipURL)
	if err != nil {
		return "", fmt.Errorf("download ffmpeg: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download ffmpeg: server returned %s", resp.Status)
	}

	sendProgress(progress, "extracting...")

	// Read full response into memory to extract zip.
	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read ffmpeg zip: %w", err)
	}

	zipReader, err := zip.NewReader(io.NewSectionReader(
		readerAtBytes(zipData), 0, int64(len(zipData)),
	), int64(len(zipData)))
	if err != nil {
		return "", fmt.Errorf("open ffmpeg zip: %w", err)
	}

	// Find the ffmpeg binary inside the zip.
	var ffmpegZipFile *zip.File
	for _, f := range zipReader.File {
		// The zip typically contains just "ffmpeg" (a Mach-O binary).
		if !f.FileInfo().IsDir() && (f.Name == "ffmpeg" || strings.HasSuffix(f.Name, "/ffmpeg")) {
			ffmpegZipFile = f
			break
		}
	}
	if ffmpegZipFile == nil {
		return "", errors.New("ffmpeg binary not found in zip archive")
	}

	// Create target directory.
	destDir := filepath.Join(dataDir, "ffmpeg")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create ffmpeg directory: %w", err)
	}

	destPath := filepath.Join(destDir, "ffmpeg")

	// Extract the binary.
	if err := extractZipFile(ffmpegZipFile, destPath); err != nil {
		return "", fmt.Errorf("extract ffmpeg: %w", err)
	}

	// Make executable.
	if err := os.Chmod(destPath, 0o755); err != nil {
		return "", fmt.Errorf("chmod ffmpeg: %w", err)
	}

	sendProgress(progress, "ffmpeg ready")
	return destPath, nil
}

// sendProgress safely sends a status string on the progress channel.
// It is a no-op if the channel is nil.
func sendProgress(ch chan<- string, msg string) {
	if ch != nil {
		ch <- msg
	}
}

// extractZipFile extracts a single zip.File to destPath.
func extractZipFile(f *zip.File, destPath string) error {
	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open zip entry: %w", err)
	}
	defer rc.Close()

	out, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	if err != nil {
		return fmt.Errorf("write output file: %w", err)
	}
	return nil
}

// readerAtBytes implements io.ReaderAt over a byte slice.
type readerAtBytes []byte

func (b readerAtBytes) ReadAt(p []byte, off int64) (int, error) {
	if off >= int64(len(b)) {
		return 0, io.EOF
	}
	n := copy(p, b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
