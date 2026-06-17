# Spo3fy — Spotify Downloader (no credentials needed)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Spo3fy downloads Spotify tracks, albums, and playlists as MP3 (or other formats) with full metadata and cover art — no API credentials or premium subscription required. Scrapes public Spotify pages, uses **yt-dlp** for audio extraction, and falls back to the **iTunes Search API** for track metadata when needed.

## Features

- **TUI mode** — centered Bubble Tea interface with slash commands, search, progress bars, and results view
- **CLI mode** — headless download by passing a URL directly: `spo3fy "https://open.spotify.com/track/..."`
- **Smart URL paste** — paste detection in TUI auto-recognizes Spotify links
- **Format support** — MP3, AAC, FLAC, M4A, Opus, Vorbis, WAV
- **Quality presets** — best, 320k, 256k, 192k, 128k, 96k, worst
- **Cover art embedding** — auto-fetches and embeds album art into audio files
- **M3U playlists** — optional extended M3U generation for batch downloads
- **Output grouping** — organize downloads into subdirectories with `--group`
- **Single static binary** — ~11 MB, zero runtime deps beyond yt-dlp/ffmpeg

## Prerequisites

**yt-dlp** and **ffmpeg** must be on your `PATH`. Install them once:

```shell
# macOS (Homebrew)
brew install yt-dlp ffmpeg

# Linux (apt)
sudo apt install yt-dlp ffmpeg

# Any platform (pip)
pip install yt-dlp
```

Verify with `yt-dlp --version` and `ffmpeg -version`.

## Installation

**Go 1.26+** is required for `go install`.

**Option 1 — `go install`** (recommended):
```shell
go install github.com/whasbe1s5/Spo3fy@latest
```
This places `spo3fy` in your Go bin directory. Make sure that directory is on your `PATH`:
```shell
export PATH="$(go env GOPATH)/bin:$PATH"   # add to ~/.zshrc or ~/.bashrc
```

**Option 2 — manual** (from source):
```shell
git clone https://github.com/whasbe1s5/Spo3fy.git
cd Spo3fy
go build -o spo3fy .
cp spo3fy ~/.local/bin/    # or /usr/local/bin/, or any directory on PATH
```

## Quick Start

```shell
# Launch TUI
spo3fy

# Download a single track (headless)
spo3fy "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT"

# Download an album
spo3fy "https://open.spotify.com/album/1A2GTWGtFfWp7KSQTwWOyo"

# Download a playlist with M3U and 320k quality
spo3fy --quality 320k --m3u "https://open.spotify.com/playlist/..."
```

## Usage

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--type` | `-t` | `track` | Resource type: `track`, `album`, `playlist`, `artist` |
| `--quality` | `-q` | `best` | Audio quality: `best`, `320k`, `256k`, `192k`, `128k`, `96k`, `worst` |
| `--format` | `-f` | `mp3` | Output format: `mp3`, `aac`, `flac`, `m4a`, `opus`, `vorbis`, `wav` |
| `--output` | `-o` | `~/Music/Spo3fy` | Output directory |
| `--m3u` | `-m` | `false` | Create an M3U playlist file |
| `--group` | `-g` | | Grouping subdirectory name for output |
| `--verbose` | `-v` | `false` | Verbose logging (tag/cover errors) |

## TUI Walkthrough

Running `spo3fy` without arguments opens the TUI:

1. **Settings screen** — choose output format (mp3/flac/aac/…), quality, cover art embedding, and M3U generation via number keys
2. **Search/download** — paste a Spotify URL or type a search query; the TUI auto-detects URLs and resolves them to track listings
3. **Progress screen** — per-track progress bars showing download status
4. **Results screen** — summary of completed downloads with file paths

**Slash commands** — type `/` to open the command palette: `/help`, `/search <q>`, `/output <path>`, `/quality <val>`, `/format <val>`, `/quit`.

## FAQ

**Does this need Spotify Premium?** No. It works from public Spotify pages without authentication.

**Does it need API keys?** No. Everything is scraped from public pages or resolved via the iTunes Search API.

**What audio formats are available?** MP3 (default), AAC, FLAC, M4A, Opus, Vorbis, and WAV.

**Does it use a lot of bandwidth?** yt-dlp extracts audio from YouTube-equivalent sources — typical downloads are a few MB per track.

## Architecture

```
Spotify URL → Scraper (public page + iTunes fallback)
                  ↓
          Track metadata + cover art URL
                  ↓
         yt-dlp (ytsearch audio download)
                  ↓
            FFmpeg (tagging + cover embed)
                  ↓
            Output file + M3U (optional)
```

The scraper resolves Spotify URLs to track metadata using public HTML pages. If metadata is incomplete, the iTunes Search API fills gaps. Audio is located via yt-dlp's `ytsearch:` feature, then tagged with FFmpeg for ID3/metadata and cover art.

## Disclaimer

Spo3fy is **not affiliated, associated, authorized, endorsed by, or in any way officially connected with Spotify AB**. This tool is for personal use only. Users are responsible for complying with applicable copyright laws and terms of service.

## License

[MIT](LICENSE)
