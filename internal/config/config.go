// Package config handles path resolution, directory creation and cleanup.
package config

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
)

// Defaults for path handling.
const (
	AppName     = "spo3fy"
	DefaultDir  = ".local/share/spo3fy" // relative to user home
	DefaultOut  = "Music/Spo3fy"        // relative to user home
)

// Paths holds resolved paths for the current run.
type Paths struct {
	DataDir string // ~/.local/share/spo3fy
	OutDir  string // ~/Music/Spo3fy
	TempDir string // DataDir/tmp
	LogDir  string // DataDir/logs
}

// Resolve returns a Paths with the given overrides applied.
// If dataDir or outDir are empty, defaults are used.
func Resolve(dataDir, outDir string) *Paths {
	home := homeDir()
	if dataDir == "" {
		dataDir = filepath.Join(home, DefaultDir)
	}
	if outDir == "" {
		outDir = filepath.Join(home, DefaultOut)
	}
	return &Paths{
		DataDir: dataDir,
		OutDir:  outDir,
		TempDir: filepath.Join(dataDir, "tmp"),
		LogDir:  filepath.Join(dataDir, "logs"),
	}
}

// EnsureDirs creates all required directories.
func (p *Paths) EnsureDirs() error {
	dirs := []string{p.DataDir, p.OutDir, p.TempDir, p.LogDir}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}
	return nil
}

// CleanTemp removes all files in the temp directory.
func (p *Paths) CleanTemp() error {
	return os.RemoveAll(p.TempDir)
}

// OutputPath returns the full output path for a track.
func (p *Paths) OutputPath(trackName string, ext string) string {
	name := SafeFilename(trackName)
	return filepath.Join(p.OutDir, name+"."+ext)
}

func homeDir() string {
	if u, err := user.Current(); err == nil {
		return u.HomeDir
	}
	return os.Getenv("HOME")
}

// SafeFilename replaces characters unsafe for filesystem names.
func SafeFilename(name string) string {
	result := make([]byte, 0, len(name))
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == ' ' || c == '-' || c == '_' || c == '.' || c == ',' || c == '(' || c == ')' {
			result = append(result, c)
		} else {
			result = append(result, '_')
		}
	}
	// Trim trailing periods (Windows compatibility)
	n := len(result)
	for n > 0 && result[n-1] == '.' {
		n--
	}
	// Cap at 255 bytes
	if n > 255 {
		n = 255
	}
	return string(result[:n])
}

// IsARM returns true on Apple Silicon or other ARM64.
func IsARM() bool {
	return runtime.GOARCH == "arm64"
}

// IsWindows returns true on Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
