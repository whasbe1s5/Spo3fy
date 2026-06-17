# Spo3fy — Spotify Downloader

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Spo3fy downloads Spotify tracks, albums, and playlists as MP3 (or other formats) with full metadata and cover art — no API credentials or premium subscription required.

Albums and playlists are saved into their own subfolder with the cover image alongside the tracks.

## Prerequisites

**yt-dlp** and **ffmpeg** must be on your `PATH`. Install once:

```shell
# macOS
brew install yt-dlp ffmpeg

# Linux (apt)
sudo apt install yt-dlp ffmpeg

# Any platform (pip)
pip install yt-dlp
```

Verify: `yt-dlp --version` and `ffmpeg -version`.

## Install

```shell
go install github.com/whasbe1s5/Spo3fy@latest
```

Ensure `$GOBIN` is on your `PATH` (default: `~/go/bin` on macOS/Linux).

To bypass the Go module proxy cache:

```shell
GOPROXY=direct go install github.com/whasbe1s5/Spo3fy@latest
```

## Quick Start

```shell
# Single track
spo3fy "https://open.spotify.com/track/4cOdK2wGLETKBW3PvgPWqT"

# Album — downloaded into ~/Music/Spo3fy/Album Name/
spo3fy "https://open.spotify.com/album/0sNOF9WDwhWunNAHPD3Baj"

# Playlist — downloaded into ~/Music/Spo3fy/Playlist Name/
spo3fy "https://open.spotify.com/playlist/37i9dQZEVXbMDoHDwVN2tF"

# With quality, format, verbose logging
spo3fy -q 320k -f flac -v "https://open.spotify.com/track/..."
```

## Usage

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--type` | `-t` | `track` | Resource type: `track`, `album`, `playlist`, `artist` |
| `--quality` | `-q` | `best` | Audio quality: `best`, `320k`, `256k`, `192k`, `128k`, `96k`, `worst` |
| `--format` | `-f` | `mp3` | Output format: `mp3`, `aac`, `flac`, `m4a`, `opus`, `vorbis`, `wav` |
| `--output` | `-o` | `~/Music/Spo3fy` | Output directory |
| `--group` | `-g` | | Override subfolder name (default: album/playlist name) |
| `--m3u` | `-m` | `false` | Create M3U playlist file in output folder |
| `--all-albums` | `-a` | `false` | Download all albums for an artist |
| `--skip-cover-art` | | `false` | Skip per-track cover art embedding |
| `--verbose` | `-v` | `false` | Show tagging and cover art errors |

## How It Works

```
Spotify URL → Scraper (public embed pages + iTunes Search API fallback)
                  ↓
       Track list + album/playlist cover URL
                  ↓
         yt-dlp (ytsearch: audio extraction)
                  ↓
            FFmpeg (ID3/metadata tagging + cover art embedding)
                  ↓
       Output folder: tracks + cover.jpg + playlist.m3u (optional)
```

The scraper resolves Spotify URLs using public HTML embed pages. If per-track metadata is sparse (common for playlists), the iTunes Search API fills gaps. Audio is located via yt-dlp's `ytsearch:` feature, then tagged with FFmpeg.

## FAQ

**Does this need Spotify Premium?** No. Public pages only — no authentication.

**Does it need API keys?** No. All metadata from public pages or the iTunes Search API.

**What formats?** MP3 (default), AAC, FLAC, M4A, Opus, Vorbis, WAV.

**Where do files go?** `~/Music/Spo3fy/` by default. Albums and playlists get their own subfolder.

**Does it download cover art?** Yes — per-track cover art is embedded in each audio file, and album/playlist covers are saved as `cover.jpg` in the output folder.

## Disclaimer

Spo3fy is **not affiliated with Spotify AB**. This tool is for personal use only. Users are responsible for complying with applicable copyright laws and terms of service.

## License

[MIT](LICENSE)
