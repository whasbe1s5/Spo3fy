# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-06-17

### Removed

- **TUI mode** — the Bubble Tea terminal UI has been removed. Spo3fy is CLI-only. Running `spo3fy` without arguments now shows help.

### Added

- **Album/playlist folder grouping** — tracks land in a subfolder named after the album or playlist (e.g. `~/Music/Spo3fy/She's So Unusual/`).
- **Container cover image download** — album and playlist cover art is saved as `cover.jpg` alongside the downloaded tracks.
- `LinkWithMeta` scraper function returning container name, image URL, and type alongside tracks.
- Warning display for partially-successful downloads (tracks that downloaded but failed cover art embedding).

### Fixed

- **Playlist cover art always default** — the Spotify embed page uses `coverArt.sources[].url` for playlist covers, but the struct only parsed `visualIdentity.image[]` which is absent for playlist entities. All playlist tracks falling back from iTunes Search received the default Spotify icon, causing cover art to be silently skipped.
- **Tilde expansion in output paths** — `~/Music/Spo3fy` is now correctly expanded to the user's home directory.
- **Duplicate cover download logic** — CLI and TUI (while it existed) used separate functions for downloading cover art. Unified into `tagger.DownloadAndEmbedCover`.

### Changed

- `--group` flag now overrides the default album/playlist subfolder name (previously it was the only way to get a subfolder).
- Output directory defaults to empty string in code (resolved to `~/Music/Spo3fy` by `config.Resolve`), fixing tilde handling.

## [0.1.2] - 2026-06-17

### Fixed

- Playlist downloads with no cover art: when `ScrapeTrack` fails for individual playlist tracks, the per-track `CoverArtURL` now falls back to the playlist's own cover image.
- Track metadata scrape: reverted to custom `Spo3fy` User-Agent because a standard browser User-Agent causes Spotify to serve a client-side SPA shell lacking SEO metadata tags.
- Playlist cover art extraction: added fallback selectors to retrieve playlist cover art from embed pages.

## [0.1.1] - 2026-06-17

### Fixed

- Cover art embedding: ffmpeg couldn't determine output format from `.tmp` extension. Temporary files now carry the proper extension (e.g. `.tmp.mp3`).

## [0.1.0] - 2025-06-17

### Added

- Bubble Tea TUI with centered views and slash commands.
- Spotify track download via yt-dlp from public pages (no credentials).
- iTunes Search API fallback for metadata.
- CLI mode for headless downloads.
- Static single binary (~10 MB) with no runtime dependencies beyond yt-dlp and ffmpeg.

### Fixed

- CLI deadlock on download completion.
- Command injection via unsanitized metadata in yt-dlp args.
- SSRF via unvalidated cover art redirects.
- Double file extension on output filenames.

[0.1.0]: https://github.com/whasbe1s5/Spo3fy/releases/tag/v0.1.0
[0.1.1]: https://github.com/whasbe1s5/Spo3fy/releases/tag/v0.1.1
[0.1.2]: https://github.com/whasbe1s5/Spo3fy/releases/tag/v0.1.2
[0.2.0]: https://github.com/whasbe1s5/Spo3fy/releases/tag/v0.2.0
