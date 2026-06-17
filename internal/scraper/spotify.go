// Package scraper provides functions for scraping Spotify metadata and searching iTunes.
package scraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/whasbe1s5/Spo3fy/internal/types"
)

const (
	spotifyBaseURL = "https://open.spotify.com"
	itunesBaseURL  = "https://itunes.apple.com/search"
	httpTimeout    = 15 * time.Second
)

var httpClient = &http.Client{Timeout: httpTimeout}

var (
	spotifyURIRE   = regexp.MustCompile(`^spotify:(track|album|playlist|artist):(.+)$`)
	trackListRE    = regexp.MustCompile(`"trackList":(\[.*?\]),`)
	defaultArtists = []string{"Unknown Artist"}
)

// ---- iTunes API response types ----

type itunesResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []itunesResult `json:"results"`
}

type itunesResult struct {
	TrackID         int    `json:"trackId"`
	TrackName       string `json:"trackName"`
	ArtistName      string `json:"artistName"`
	CollectionName  string `json:"collectionName"`
	ArtworkURL100   string `json:"artworkUrl100"`
	ArtworkURL60    string `json:"artworkUrl60"`
	TrackNumber     int    `json:"trackNumber"`
	DiscNumber      int    `json:"discNumber"`
	TrackTimeMillis int    `json:"trackTimeMillis"`
	ReleaseDate     string `json:"releaseDate"`
	TrackViewURL    string `json:"trackViewUrl"`
}

// ---- Embed page JSON types (playlist track list) ----

type embedTrackEntry struct {
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Duration int    `json:"duration"`
	URI      string `json:"uri"`
}

// ---- Embed page JSON types (track details fallback) ----

type embedTrackJSON struct {
	Props struct {
		PageProps struct {
			State struct {
				Data struct {
					Entity struct {
						Name    string `json:"name"`
						Title   string `json:"title"`
						Artists []struct {
							Name string `json:"name"`
						} `json:"artists"`
						Duration    int `json:"duration"`
						ReleaseDate struct {
							IsoString string `json:"isoString"`
						} `json:"releaseDate"`
						VisualIdentity struct {
							Image []struct {
								URL      string `json:"url"`
								MaxWidth int    `json:"maxWidth"`
							} `json:"image"`
						} `json:"visualIdentity"`
					} `json:"entity"`
				} `json:"data"`
			} `json:"state"`
		} `json:"pageProps"`
	} `json:"props"`
}

func scrapeTrackEmbed(trackID string) *types.Track {
	embedURL := spotifyBaseURL + "/embed/track/" + trackID
	doc, err := fetchPage(embedURL)
	if err != nil {
		return nil
	}

	var trackData embedTrackJSON
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "props") && strings.Contains(text, "pageProps") {
			json.Unmarshal([]byte(text), &trackData)
		}
	})

	entity := trackData.Props.PageProps.State.Data.Entity
	if entity.Name == "" && entity.Title == "" {
		return nil
	}

	name := entity.Name
	if name == "" {
		name = entity.Title
	}

	var artists []string
	for _, a := range entity.Artists {
		if a.Name != "" {
			artists = append(artists, a.Name)
		}
	}
	if len(artists) == 0 {
		artists = defaultArtists
	}

	coverURL := ""
	for _, img := range entity.VisualIdentity.Image {
		if img.MaxWidth == 640 {
			coverURL = img.URL
			break
		}
	}
	if coverURL == "" && len(entity.VisualIdentity.Image) > 0 {
		coverURL = entity.VisualIdentity.Image[0].URL
	}
	if coverURL == "" {
		coverURL = types.DefaultCoverArtURL
	}

	releaseDate := ""
	if entity.ReleaseDate.IsoString != "" {
		parts := strings.Split(entity.ReleaseDate.IsoString, "T")
		releaseDate = parts[0]
	}

	t := &types.Track{
		ID:          trackID,
		Name:        name,
		URL:         spotifyBaseURL + "/track/" + trackID,
		URI:         "spotify:track:" + trackID,
		Artists:     artists,
		DurationMS:  entity.Duration,
		ReleaseDate: releaseDate,
		CoverArtURL: coverURL,
		AlbumName:   "Unknown Album",
		Type:        types.TypeTrack,
	}

	return t
}

type embedContainerJSON struct {
	Props struct {
		PageProps struct {
			State struct {
				Data struct {
					Entity struct {
						Name           string `json:"name"`
						Title          string `json:"title"`
						VisualIdentity struct {
							Image []struct {
								URL      string `json:"url"`
								MaxWidth int    `json:"maxWidth"`
							} `json:"image"`
						} `json:"visualIdentity"`
						CoverArt struct {
							Sources []struct {
								URL    string `json:"url"`
								Width  *int   `json:"width"`
								Height *int   `json:"height"`
							} `json:"sources"`
						} `json:"coverArt"`
					} `json:"entity"`
				} `json:"data"`
			} `json:"state"`
		} `json:"pageProps"`
	} `json:"props"`
}

// resolveCoverURL picks the best cover image from a Spotify embed entity.
// Checks visualIdentity.image first (tracks/albums), then coverArt.sources (playlists).
func resolveCoverURL(e embedContainerJSON) string {
	entity := e.Props.PageProps.State.Data.Entity
	for _, img := range entity.VisualIdentity.Image {
		if img.MaxWidth == 640 {
			return img.URL
		}
	}
	if len(entity.VisualIdentity.Image) > 0 {
		return entity.VisualIdentity.Image[0].URL
	}
	for _, src := range entity.CoverArt.Sources {
		if src.URL != "" {
			return src.URL
		}
	}
	return types.DefaultCoverArtURL
}

// ---- HTTP helpers ----

// fetchPage fetches a URL and returns a goquery document.
func fetchPage(pageURL string) (*goquery.Document, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Spo3fy/1.0)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return goquery.NewDocumentFromReader(resp.Body)
}

// fetchRaw fetches a URL and returns the raw body bytes.
func fetchRaw(pageURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Spo3fy/1.0)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ---- Meta tag helpers ----

// metaContent returns the content of the first <meta> tag matching property or name.
func metaContent(doc *goquery.Document, attr, value string) string {
	sel := fmt.Sprintf(`meta[%s="%s"]`, attr, value)
	content, exists := doc.Find(sel).Attr("content")
	if !exists {
		return ""
	}
	return content
}

// metaContents returns the content of all <meta> tags matching the given attribute/value pair.
func metaContents(doc *goquery.Document, attr, value string) []string {
	var result []string
	sel := fmt.Sprintf(`meta[%s="%s"]`, attr, value)
	doc.Find(sel).Each(func(i int, s *goquery.Selection) {
		content, exists := s.Attr("content")
		if exists {
			result = append(result, content)
		}
	})
	return result
}

// ---- Public API ----

// ExtractSpotifyID extracts a Spotify ID from a URL or spotify: URI.
func ExtractSpotifyID(urlOrID string) string {
	if !strings.ContainsAny(urlOrID, ":/") {
		return urlOrID
	}
	if matches := spotifyURIRE.FindStringSubmatch(urlOrID); len(matches) >= 3 {
		return matches[2]
	}
	parsed, err := url.Parse(urlOrID)
	if err != nil {
		clean := strings.TrimRight(urlOrID, "?#/")
		if idx := strings.LastIndex(clean, "/"); idx >= 0 {
			return clean[idx+1:]
		}
		return urlOrID
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) > 0 && parts[len(parts)-1] != "" {
		return parts[len(parts)-1]
	}
	return parsed.Path
}

// ScrapeTrack scrapes track metadata from open.spotify.com/track/<id>.
func ScrapeTrack(trackID string) *types.Track {
	pageURL := spotifyBaseURL + "/track/" + trackID
	doc, err := fetchPage(pageURL)
	if err != nil || metaContent(doc, "property", "og:title") == "" {
		return scrapeTrackEmbed(trackID)
	}

	track := &types.Track{
		ID:   trackID,
		URL:  pageURL,
		URI:  "spotify:track:" + trackID,
		Type: types.TypeTrack,
	}

	// og:title — track name
	if title := metaContent(doc, "property", "og:title"); title != "" {
		track.Name = title
	}

	// og:image — cover art
	if img := metaContent(doc, "property", "og:image:secure_url"); img != "" {
		track.CoverArtURL = img
	} else if img := metaContent(doc, "property", "og:image"); img != "" {
		track.CoverArtURL = img
	}

	// og:description — "Artist · Album · Year" format
	if desc := metaContent(doc, "property", "og:description"); desc != "" {
		parts := strings.Split(desc, " · ")
		if len(parts) >= 1 {
			track.Artists = []string{strings.TrimSpace(parts[0])}
		}
		if len(parts) >= 2 {
			track.AlbumName = strings.TrimSpace(parts[1])
		}
		if len(parts) >= 3 {
			year := strings.TrimSpace(parts[2])
			if year != "" {
				track.ReleaseDate = year
			}
		}
	}

	// music:album:track — track number
	if numStr := metaContent(doc, "name", "music:album:track"); numStr != "" {
		if num, err := strconv.Atoi(numStr); err == nil {
			track.TrackNumber = num
		}
	}

	// music:duration — in seconds, convert to milliseconds
	if durStr := metaContent(doc, "name", "music:duration"); durStr != "" {
		if sec, err := strconv.Atoi(durStr); err == nil {
			track.DurationMS = sec * 1000
		}
	}

	// music:release_date
	if rel := metaContent(doc, "name", "music:release_date"); rel != "" {
		track.ReleaseDate = rel
	}

	// Fill defaults
	if track.Name == "" {
		track.Name = "Unknown Track"
	}
	if len(track.Artists) == 0 {
		track.Artists = defaultArtists
	}
	if track.AlbumName == "" {
		track.AlbumName = "Unknown Album"
	}
	if track.CoverArtURL == "" {
		track.CoverArtURL = types.DefaultCoverArtURL
	}

	return track
}

// ScrapeAlbum scrapes album metadata + track list from open.spotify.com/album/<id>.
func ScrapeAlbum(albumID string) (*types.Album, error) {
	embedURL := spotifyBaseURL + "/embed/album/" + albumID
	body, err := fetchRaw(embedURL)
	if err != nil {
		return nil, fmt.Errorf("scraping album %s: %w", albumID, err)
	}

	album := &types.Album{
		ID:  albumID,
		URL: spotifyBaseURL + "/album/" + albumID,
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	var containerData embedContainerJSON
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "props") && strings.Contains(text, "pageProps") {
			json.Unmarshal([]byte(text), &containerData)
		}
	})

	entity := containerData.Props.PageProps.State.Data.Entity
	album.Name = entity.Name
	if album.Name == "" {
		album.Name = entity.Title
	}
	if album.Name == "" {
		album.Name = "Unknown Album"
	}

	album.ImageURL = resolveCoverURL(containerData)

	entries := tryParseEmbedJSON(body)
	album.TotalTracks = len(entries)
	album.Tracks = make([]types.Track, 0, len(entries))

	for i, entry := range entries {
		t := buildMinimalTrack(entry)
		t.AlbumName = album.Name
		t.CoverArtURL = album.ImageURL
		t.TrackNumber = i + 1
		album.Tracks = append(album.Tracks, *t)
	}

	if desc := metaContent(doc, "property", "og:description"); desc != "" {
		parts := strings.Split(desc, " · ")
		if len(parts) >= 1 {
			album.Artists = []string{strings.TrimSpace(parts[0])}
		}
		if len(parts) >= 3 {
			album.ReleaseDate = strings.TrimSpace(parts[2])
		}
	}
	if len(album.Artists) == 0 {
		album.Artists = defaultArtists
	}

	return album, nil
}

func ScrapePlaylist(playlistID string) (*types.Playlist, error) {
	embedURL := spotifyBaseURL + "/embed/playlist/" + playlistID
	body, err := fetchRaw(embedURL)
	if err != nil {
		return nil, fmt.Errorf("scraping playlist %s: %w", playlistID, err)
	}

	playlist := &types.Playlist{
		ID:  playlistID,
		URL: spotifyBaseURL + "/playlist/" + playlistID,
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(string(body)))
	if err != nil {
		return nil, err
	}

	var containerData embedContainerJSON
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "props") && strings.Contains(text, "pageProps") {
			json.Unmarshal([]byte(text), &containerData)
		}
	})

	entity := containerData.Props.PageProps.State.Data.Entity
	playlist.Name = entity.Name
	if playlist.Name == "" {
		playlist.Name = entity.Title
	}
	if playlist.Name == "" {
		playlist.Name = "Unknown Playlist"
	}

	playlist.ImageURL = resolveCoverURL(containerData)

	entries := tryParseEmbedJSON(body)
	playlist.Tracks = make([]types.Track, len(entries))
	var wg sync.WaitGroup
	for i, entry := range entries {
		wg.Add(1)
		go func(idx int, ent embedTrackEntry) {
			defer wg.Done()
			t := buildMinimalTrack(ent)
			t.Playlist = playlistID

			// Attempt to resolve individual track metadata from iTunes Search API in parallel
			searchTerm := t.Name
			if len(t.Artists) > 0 {
				searchTerm += " " + t.Artists[0]
			}
			results, err := Search(searchTerm, types.TypeTrack)
			if err == nil && len(results) > 0 {
				best := results[0]
				t.AlbumName = best.AlbumName
				t.CoverArtURL = best.CoverArtURL
				t.ReleaseDate = best.ReleaseDate
			} else {
				t.CoverArtURL = playlist.ImageURL
				t.AlbumName = "Unknown Album"
			}
			playlist.Tracks[idx] = *t
		}(i, entry)
	}
	wg.Wait()

	return playlist, nil
}

func fallbackScrapePlaylist(playlistID string) (*types.Playlist, error) {
	pageURL := spotifyBaseURL + "/playlist/" + playlistID
	doc, err := fetchPage(pageURL)
	if err != nil {
		return nil, fmt.Errorf("scraping playlist %s: %w", playlistID, err)
	}

	playlist := &types.Playlist{
		ID:  playlistID,
		URL: pageURL,
	}

	playlist.Name = extractPlaylistName(doc, nil)
	playlist.Description = extractPlaylistDescription(doc)
	if img := metaContent(doc, "property", "og:image:secure_url"); img != "" {
		playlist.ImageURL = img
	} else if img := metaContent(doc, "property", "og:image"); img != "" {
		playlist.ImageURL = img
	}
	if playlist.ImageURL == "" {
		playlist.ImageURL = types.DefaultCoverArtURL
	}
	if playlist.Name == "" {
		playlist.Name = "Unknown Playlist"
	}

	// Track list from music:song meta tags (max 30)
	trackURLs := metaContents(doc, "name", "music:song")
	if len(trackURLs) > 30 {
		trackURLs = trackURLs[:30]
	}

	playlist.Tracks = make([]types.Track, 0, len(trackURLs))
	for i, tu := range trackURLs {
		tid := ExtractSpotifyID(tu)
		if tid == "" {
			continue
		}
		t := &types.Track{
			ID:          tid,
			Name:        fmt.Sprintf("Track %d", i+1),
			Artists:     defaultArtists,
			URL:         spotifyBaseURL + "/track/" + tid,
			URI:         "spotify:track:" + tid,
			Type:        types.TypeTrack,
			Playlist:    playlistID,
			CoverArtURL: playlist.ImageURL,
		}
		playlist.Tracks = append(playlist.Tracks, *t)
	}

	return playlist, nil
}

// tryParseEmbedJSON attempts to parse the playlist embed JSON from the page body.
// Returns the raw track entries (flat JSON: title, subtitle, uri, duration).
func tryParseEmbedJSON(body []byte) []embedTrackEntry {
	matches := trackListRE.FindSubmatch(body)
	if len(matches) < 2 {
		return nil
	}
	var entries []embedTrackEntry
	if err := json.Unmarshal(matches[1], &entries); err != nil {
		return nil
	}
	return entries
}
func buildMinimalTrack(entry embedTrackEntry) *types.Track {
	tid := ExtractSpotifyID(entry.URI)
	artists := parseArtistsFromSubtitle(entry.Subtitle)
	t := &types.Track{
		ID:          tid,
		Name:        defaultString(entry.Title, "Unknown Track"),
		URL:         spotifyBaseURL + "/track/" + tid,
		URI:         entry.URI,
		DurationMS:  entry.Duration,
		AlbumName:   "Unknown Album",
		CoverArtURL: types.DefaultCoverArtURL,
		Artists:     artists,
		Type:        types.TypeTrack,
	}
	return t
}

// parseArtistsFromSubtitle splits the subtitle (e.g. "Artist1, Artist2") into names.
func parseArtistsFromSubtitle(subtitle string) []string {
	if subtitle == "" {
		return defaultArtists
	}
	parts := strings.Split(subtitle, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		if n := strings.TrimSpace(p); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return defaultArtists
	}
	return names
}

func defaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

// extractPlaylistName extracts the playlist name from og:title or <title>.
func extractPlaylistName(doc *goquery.Document, body []byte) string {
	if doc != nil {
		if title := metaContent(doc, "property", "og:title"); title != "" {
			title = strings.TrimSuffix(title, " — Spotify")
			title = strings.TrimSuffix(title, " | Spotify")
			return strings.TrimSpace(title)
		}
	}
	if body != nil {
		re := regexp.MustCompile(`<title>([^<]+)</title>`)
		if m := re.FindSubmatch(body); len(m) >= 2 {
			t := strings.TrimSpace(string(m[1]))
			t = strings.TrimSuffix(t, " — Spotify")
			t = strings.TrimSuffix(t, " | Spotify")
			return strings.TrimSpace(t)
		}
	}
	return "Unknown Playlist"
}

// extractPlaylistDescription extracts the playlist description from og:description or name:description.
func extractPlaylistDescription(doc *goquery.Document) string {
	if doc != nil {
		if desc := metaContent(doc, "property", "og:description"); desc != "" {
			return desc
		}
		if desc := metaContent(doc, "name", "description"); desc != "" {
			return desc
		}
	}
	return ""
}

// Search searches the iTunes Search API (no auth needed).
func Search(query string, queryType types.Type) ([]types.Track, error) {
	params := url.Values{}
	params.Set("term", query)
	params.Set("limit", "20")

	if queryType == types.TypeArtist {
		params.Set("entity", "allArtist")
		params.Set("limit", "5")
	} else {
		params.Set("entity", "song")
	}

	reqURL := itunesBaseURL + "?" + params.Encode()
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Spo3fy/1.0)")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("iTunes search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("iTunes search HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("iTunes search read: %w", err)
	}

	var itunesResp itunesResponse
	if err := json.Unmarshal(body, &itunesResp); err != nil {
		return nil, fmt.Errorf("iTunes search decode: %w", err)
	}

	tracks := make([]types.Track, 0, itunesResp.ResultCount)
	for _, r := range itunesResp.Results {
		track := types.Track{
			ID:          strconv.Itoa(r.TrackID),
			Name:        r.TrackName,
			URL:         r.TrackViewURL,
			Artists:     []string{r.ArtistName},
			AlbumName:   r.CollectionName,
			TrackNumber: r.TrackNumber,
			DiscNumber:  r.DiscNumber,
			DurationMS:  r.TrackTimeMillis,
			ReleaseDate: cleanDate(r.ReleaseDate),
			Type:        types.TypeTrack,
		}
		// Upscale artwork URL to 640x640
		if r.ArtworkURL100 != "" {
			track.CoverArtURL = strings.Replace(r.ArtworkURL100, "100x100", "640x640", 1)
		} else if r.ArtworkURL60 != "" {
			track.CoverArtURL = strings.Replace(r.ArtworkURL60, "60x60", "640x640", 1)
		} else {
			track.CoverArtURL = types.DefaultCoverArtURL
		}
		if track.Name == "" {
			track.Name = "Unknown Track"
		}
		if track.AlbumName == "" {
			track.AlbumName = "Unknown Album"
		}
		if len(track.Artists) == 0 {
			track.Artists = defaultArtists
		}
		tracks = append(tracks, track)
	}

	return tracks, nil
}

// cleanDate normalizes an iTunes ISO date to YYYY-MM-DD.
func cleanDate(date string) string {
	if date == "" {
		return ""
	}
	if idx := strings.Index(date, "T"); idx >= 0 {
		return date[:idx]
	}
	return date
}

// Link resolves ANY Spotify URL (track/album/playlist/artist) to tracks.
func Link(query string) ([]types.Track, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, errors.New("empty query")
	}

	resourceType, resourceID := parseSpotifyResource(query)
	if resourceID == "" {
		return Search(query, types.TypeTrack)
	}

	switch resourceType {
	case "track":
		t := ScrapeTrack(resourceID)
		if t == nil {
			return nil, fmt.Errorf("failed to scrape track: %s", resourceID)
		}
		return []types.Track{*t}, nil

	case "album":
		album, err := ScrapeAlbum(resourceID)
		if err != nil {
			return nil, err
		}
		if len(album.Tracks) == 0 {
			return []types.Track{}, nil
		}
		return album.Tracks, nil

	case "playlist":
		playlist, err := ScrapePlaylist(resourceID)
		if err != nil {
			return nil, err
		}
		if len(playlist.Tracks) == 0 {
			return []types.Track{}, nil
		}
		return playlist.Tracks, nil

	case "artist":
		return scrapeArtist(resourceID)

	default:
		return Search(query, types.TypeTrack)
	}
}

// parseSpotifyResource extracts resource type and ID from a Spotify URL or URI.
func parseSpotifyResource(input string) (string, string) {
	if matches := spotifyURIRE.FindStringSubmatch(input); len(matches) >= 3 {
		return matches[1], matches[2]
	}

	parsed, err := url.Parse(input)
	if err != nil || parsed.Host == "" {
		return "", ""
	}
	if !strings.Contains(parsed.Host, "spotify.com") {
		return "", ""
	}

	pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", ""
	}

	rtype := pathParts[0]
	id := pathParts[1]
	switch rtype {
	case "track", "album", "playlist", "artist":
		return rtype, id
	default:
		return "", ""
	}
}

// scrapeArtist scrapes an artist page for the name, then searches iTunes.
func scrapeArtist(artistID string) ([]types.Track, error) {
	pageURL := spotifyBaseURL + "/artist/" + artistID
	doc, err := fetchPage(pageURL)
	if err != nil {
		return nil, fmt.Errorf("scraping artist %s: %w", artistID, err)
	}

	artistName := ""
	if title := metaContent(doc, "property", "og:title"); title != "" {
		artistName = strings.TrimSpace(title)
	}
	if artistName == "" {
		if desc := metaContent(doc, "name", "description"); desc != "" {
			parts := strings.SplitN(desc, " · ", 2)
			artistName = strings.TrimSpace(parts[0])
		}
	}
	if artistName == "" {
		return nil, fmt.Errorf("could not determine artist name from page")
	}

	return Search(artistName, types.TypeArtist)
}
