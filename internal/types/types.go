// Package types defines the shared enums and data structures for Spo3fy.
package types

import "fmt"

// Type represents a Spotify resource type.
type Type string

const (
	TypeTrack    Type = "track"
	TypeAlbum    Type = "album"
	TypePlaylist Type = "playlist"
	TypeArtist   Type = "artist"
	TypeEpisode  Type = "episode"
)

// Quality represents audio bitrate for downloaded files.
type Quality string

const (
	QualityBest  Quality = "0"
	Quality320k  Quality = "320"
	Quality256k  Quality = "256"
	Quality192k  Quality = "192"
	Quality128k  Quality = "128"
	Quality96k   Quality = "96"
	Quality32k   Quality = "32"
	QualityWorst Quality = "9"
)

// Format represents the output audio format.
type Format string

const (
	FormatMP3    Format = "mp3"
	FormatAAC    Format = "aac"
	FormatFLAC   Format = "flac"
	FormatM4A    Format = "m4a"
	FormatOpus   Format = "opus"
	FormatVorbis Format = "vorbis"
	FormatWAV    Format = "wav"
)

// Track represents a single track with all metadata.
type Track struct {
	ID              string
	Name            string
	URL             string
	URI             string
	Artists         []string
	AlbumName       string
	AlbumTrackCount int
	TrackNumber     int
	DiscNumber      int
	DurationMS      int
	ReleaseDate     string
	CoverArtURL     string
	Playlist        string
	Type            Type
}

// String returns "Artist - Track Name".
func (t Track) String() string {
	artist := "Unknown Artist"
	if len(t.Artists) > 0 {
		artist = t.Artists[0]
	}
	return fmt.Sprintf("%s - %s", artist, t.Name)
}

// DefaultCoverArtURL is the fallback Spotify branding icon.
const DefaultCoverArtURL = "https://developer.spotify.com/assets/branding-guidelines/icon3@2x.png"

// IsDefaultCover returns true if the cover art is the default Spotify icon.
func (t Track) IsDefaultCover() bool {
	return t.CoverArtURL == DefaultCoverArtURL || t.CoverArtURL == ""
}

// Album represents album metadata.
type Album struct {
	ID          string
	Name        string
	URL         string
	Artists     []string
	ImageURL    string
	ReleaseDate string
	TotalTracks int
	Tracks      []Track
}

// Playlist represents playlist metadata with its tracks.
type Playlist struct {
	ID          string
	Name        string
	URL         string
	Description string
	ImageURL    string
	Tracks      []Track
}

// DownloadResult holds the outcome of a single-track download attempt.
type DownloadResult struct {
	Track      Track
	OutputPath string
	Success    bool
	Error      string
}

// QueryType maps a Type to a user-facing label for the CLI.
type QueryType struct {
	Type  Type
	Label string
}
