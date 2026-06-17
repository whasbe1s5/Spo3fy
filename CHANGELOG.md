# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-06-17

### Added

- Bubble Tea TUI with centered views and a blinking cursor.
- Spotify track download via yt-dlp, scraped from public Spotify pages (no API credentials needed).
- iTunes Search API fallback for metadata lookups.
- Slash command system with auto-completing dropdown and descriptions.
- Text search fallback when a URL is not provided.
- ASCII art logo and descriptive setting labels.
- Auto-detection of resource type from URL (track, album, playlist).
- Toggle and cycle interactions for settings, with field hints and an overall progress bar.
- CLI mode: headless download by passing a Spotify URL directly.
- Static single binary (Go, ~11 MB) with no runtime dependencies other than yt-dlp and ffmpeg.

### Changed

- Restricted quit to `Ctrl+Q` and `/quit` only; other keybindings no longer exit the application.

### Fixed

- **Critical fixes:**
  - CLI deadlock on download completion.
  - Command injection via unsanitized metadata in yt-dlp args.
  - SSRF via unvalidated cover art redirects.
  - State machine: keys leaking into wrong TUI states.
  - Double file extension on output filenames.

[0.1.0]: https://github.com/whasbe1s5/Spo3fy/releases/tag/v0.1.0
