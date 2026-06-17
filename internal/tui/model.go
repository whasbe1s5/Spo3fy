package tui

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/whasbe1s/spo3fy-go/internal/config"
	"github.com/whasbe1s/spo3fy-go/internal/downloader"
	"github.com/whasbe1s/spo3fy-go/internal/scraper"
	"github.com/whasbe1s/spo3fy-go/internal/tagger"
	"github.com/whasbe1s/spo3fy-go/internal/types"
)

// ── States ──────────────────────────────────────────────────────────────

type state int

const (
	stateSettings state = iota
	stateDownloading
	stateResults
)

// ── Settings ────────────────────────────────────────────────────────────

// settings holds all user-configurable options for the download session.
type settings struct {
	resourceType types.Type
	quality      types.Quality
	format       types.Format
	outputDir    string
	groupDir     bool
	createM3U    bool
	skipCoverArt bool
	allAlbums    bool
}

func defaultSettings() settings {
	return settings{
		resourceType: types.TypeTrack,
		quality:      types.QualityBest,
		format:       types.FormatMP3,
		outputDir:    "~/Music/Spo3fy",
	}
}

// ── Spinner ─────────────────────────────────────────────────────────────

type spinnerModel struct {
	frames []string
	idx    int
}

func newSpinner() spinnerModel {
	return spinnerModel{
		frames: []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"},
	}
}

func (s *spinnerModel) Tick() {
	s.idx = (s.idx + 1) % len(s.frames)
}

func (s spinnerModel) View() string {
	return s.frames[s.idx]
}

// ── Model ───────────────────────────────────────────────────────────────

// model implements tea.Model for the Spo3fy TUI.
type model struct {
	state        state
	cfg          settings
	urlInput     string
	editingField int    // 0 = not editing; 1-8 = editing that setting
	editBuffer   string // accumulated input while editing a field

	progress downloader.ProgressUpdate
	results  []types.DownloadResult
	spinner  spinnerModel

	width, height int
	err           error

	downloadsTotal int
	downloadsDone  int

	// Channels for the download goroutine to communicate back to Update.
	progressCh chan progressMsg
	resultCh   chan resultMsg
	totalCh    chan int
	errCh      chan error

	downloadRunning bool
	cursorVisible   bool
}

// ── Internal message types ──────────────────────────────────────────────

type progressMsg struct {
	downloader.ProgressUpdate
}

type resultMsg struct {
	types.DownloadResult
}

type totalMsg struct {
	count int
}

type errMsg struct {
	error
}


// cursorBlinkMsg toggles cursor visibility for URL input.
type cursorBlinkMsg time.Time
type spinnerTickMsg time.Time

// ── Constructor ─────────────────────────────────────────────────────────

// New creates and returns a new TUI model ready for tea.NewProgram.
func New() tea.Model {
	return &model{
		state:  stateSettings,
		cfg:    defaultSettings(),
		spinner: newSpinner(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg { return spinnerTickMsg(t) }),
		tea.Tick(530*time.Millisecond, func(t time.Time) tea.Msg { return cursorBlinkMsg(t) }),
	)
}

// ── Update ──────────────────────────────────────────────────────────────

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)

	case spinnerTickMsg:
		if m.state == stateDownloading {
			m.spinner.Tick()
		}
		return m, tea.Tick(80*time.Millisecond, func(t time.Time) tea.Msg {
			return spinnerTickMsg(t)
		})

	case progressMsg:
		m.progress = msg.ProgressUpdate
		return m, listenDownload(m)

	case resultMsg:
		m.results = append(m.results, msg.DownloadResult)
		m.downloadsDone++
		if m.downloadsTotal > 0 && m.downloadsDone >= m.downloadsTotal {
			m.downloadRunning = false
			m.state = stateResults
			return m, nil
		}
		return m, listenDownload(m)

	case totalMsg:
		m.downloadsTotal = msg.count
		if msg.count == 0 {
			m.downloadRunning = false
			m.state = stateResults
			return m, nil
		}
		return m, listenDownload(m)

	case errMsg:
		m.err = msg.error
		m.downloadRunning = false
		m.state = stateResults
		return m, nil
	case cursorBlinkMsg:
		m.cursorVisible = !m.cursorVisible
		return m, tea.Tick(530*time.Millisecond, func(t time.Time) tea.Msg { return cursorBlinkMsg(t) })
	}

	return m, nil
}

func (m *model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Global quit keys — always work.
	if keyMatches(msg, "ctrl+c") || keyMatches(msg, "q") {
		return m, tea.Quit
	}
	if keyMatches(msg, "esc") {
		if m.editingField > 0 {
			m.editingField = 0
			m.editBuffer = ""
			return m, nil
		}
		return m, tea.Quit
	}

	switch m.state {
	case stateSettings:
		return m.handleSettingsKeys(msg)
	case stateResults:
		return m.handleResultsKeys(msg)
	default:
		return m, nil
	}
}

// ── Settings key handling ───────────────────────────────────────────────

func (m *model) handleSettingsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.editingField > 0 {
		return m.handleEditingKeys(msg)
	}

	switch {
	case keyMatches(msg, "enter"):
		if m.urlInput == "" {
			return m, nil
		}
		return m.startDownload()

	case keyMatches(msg, "backspace"):
		if len(m.urlInput) > 0 {
			m.urlInput = m.urlInput[:len(m.urlInput)-1]
		}
		return m, nil

	case isDigitKey(msg):
		d := digitValue(msg)
		switch d {
		case 1, 2, 3:
			m.cycleField(d)
		case 5, 6, 7, 8:
			m.toggleField(d)
		case 4:
			m.editingField = 4
			m.editBuffer = ""
		}
		return m, nil

	case isRuneKey(msg):
		m.urlInput += string(msg.Runes)
		return m, nil
	}

	return m, nil
}

func (m *model) handleEditingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyMatches(msg, "enter"):
		m.applyEdit()
		m.editingField = 0
		m.editBuffer = ""
		return m, nil

	case keyMatches(msg, "esc"):
		m.editingField = 0
		m.editBuffer = ""
		return m, nil

	case keyMatches(msg, "backspace"):
		if len(m.editBuffer) > 0 {
			m.editBuffer = m.editBuffer[:len(m.editBuffer)-1]
		}
		return m, nil

	case isRuneKey(msg):
		m.editBuffer += string(msg.Runes)
		return m, nil
	}

	return m, nil
}

func (m *model) applyEdit() {
	switch m.editingField {
	case 1:
		m.cfg.resourceType = types.Type(m.editBuffer)
	case 2:
		m.cfg.quality = types.Quality(m.editBuffer)
	case 3:
		m.cfg.format = types.Format(m.editBuffer)
	case 4:
		if m.editBuffer != "" {
			m.cfg.outputDir = m.editBuffer
		}
	case 5:
		m.cfg.groupDir = isAffirmative(m.editBuffer)
	case 6:
		m.cfg.createM3U = isAffirmative(m.editBuffer)
	case 7:
		m.cfg.skipCoverArt = isAffirmative(m.editBuffer)
	case 8:
		m.cfg.allAlbums = isAffirmative(m.editBuffer)
	}
}
var typeOrder = []types.Type{types.TypeTrack, types.TypeAlbum, types.TypePlaylist, types.TypeArtist}

var qualityOrder = []types.Quality{types.QualityBest, types.Quality320k, types.Quality256k,
	types.Quality192k, types.Quality128k, types.Quality96k, types.QualityWorst}

var formatOrder = []types.Format{types.FormatMP3, types.FormatAAC, types.FormatFLAC,
	types.FormatM4A, types.FormatOpus, types.FormatVorbis, types.FormatWAV}

func cycleOrder[T comparable](items []T, current T) T {
	for i, v := range items {
		if v == current {
			return items[(i+1)%len(items)]
		}
	}
	return items[0]
}

func (m *model) cycleField(field int) {
	switch field {
	case 1:
		m.cfg.resourceType = cycleOrder(typeOrder, m.cfg.resourceType)
	case 2:
		m.cfg.quality = cycleOrder(qualityOrder, m.cfg.quality)
	case 3:
		m.cfg.format = cycleOrder(formatOrder, m.cfg.format)
	}
}

func (m *model) toggleField(field int) {
	switch field {
	case 5:
		m.cfg.groupDir = !m.cfg.groupDir
	case 6:
		m.cfg.createM3U = !m.cfg.createM3U
	case 7:
		m.cfg.skipCoverArt = !m.cfg.skipCoverArt
	case 8:
		m.cfg.allAlbums = !m.cfg.allAlbums
	}
}

// ── Results key handling ────────────────────────────────────────────────

func (m *model) handleResultsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if keyMatches(msg, "enter") {
		m.resetForSettings()
		return m, nil
	}
	return m, nil
}

// resetForSettings clears download state and returns to the settings screen.
func (m *model) resetForSettings() {
	m.state = stateSettings
	m.progress = downloader.ProgressUpdate{}
	m.results = nil
	m.err = nil
	m.downloadsTotal = 0
	m.downloadsDone = 0
	m.downloadRunning = false
}

// ── Start download ──────────────────────────────────────────────────────

// startDownload initialises channels, transitions to downloading state,
// and launches the background worker goroutine.
func (m *model) startDownload() (tea.Model, tea.Cmd) {
	m.progressCh = make(chan progressMsg, 50)
	m.resultCh = make(chan resultMsg, 50)
	m.totalCh = make(chan int, 1)
	m.errCh = make(chan error, 1)

	m.state = stateDownloading
	m.downloadRunning = true
	m.results = nil
	m.downloadsDone = 0
	m.downloadsTotal = 0
	m.err = nil

	// Snapshot the settings the goroutine should use.
	cfg := m.cfg
	urlInput := m.urlInput
	progressCh := m.progressCh
	resultCh := m.resultCh
	totalCh := m.totalCh
	errCh := m.errCh

	return m, tea.Batch(
		listenDownload(m),
		func() tea.Msg {
			downloadWorker(cfg, urlInput, progressCh, resultCh, totalCh, errCh)
			return nil
		},
	)
}

// ── Download worker ─────────────────────────────────────────────────────

// downloadWorker runs in a background goroutine. It performs the full
// download pipeline and communicates results back via channels.
func downloadWorker(
	cfg settings,
	urlInput string,
	progressCh chan<- progressMsg,
	resultCh chan<- resultMsg,
	totalCh chan<- int,
	errCh chan<- error,
) {
	// 1. Resolve paths.
	paths := config.Resolve("", cfg.outputDir)
	if err := paths.EnsureDirs(); err != nil {
		errCh <- fmt.Errorf("ensure dirs: %w", err)
		return
	}

	// 2. Ensure ffmpeg is available.
	ffmpegPath, err := ensureFFmpeg(paths.DataDir, progressCh)
	if err != nil {
		errCh <- err
		return
	}

	// 3. Resolve the Spotify link to tracks.
	tracks, err := scraper.Link(urlInput)
	if err != nil {
		errCh <- fmt.Errorf("resolve link: %w", err)
		return
	}

	totalCh <- len(tracks)

	// 4. Download, cover-art, and tag each track.
	for _, track := range tracks {
		result := processTrack(track, cfg, paths, ffmpegPath, progressCh)
		resultCh <- resultMsg{result}
	}
}

// ensureFFmpeg returns a usable ffmpeg path, downloading one if necessary.
func ensureFFmpeg(dataDir string, progressCh chan<- progressMsg) (string, error) {
	if tagger.IsAvailable("") {
		return "", nil
	}

	if path, ok := tagger.IsDownloaded(dataDir); ok {
		return path, nil
	}

	// Bridge the FFmpeg downloader's string channel to our progressMsg channel.
	fCh := make(chan string, 10)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for s := range fCh {
			progressCh <- progressMsg{downloader.ProgressUpdate{
				Status: "ffmpeg: " + s,
			}}
		}
	}()

	path, dlErr := tagger.DownloadFFmpeg(dataDir, fCh)
	close(fCh)
	wg.Wait()

	if dlErr != nil {
		return "", fmt.Errorf("download ffmpeg: %w", dlErr)
	}
	return path, nil
}

// processTrack downloads a single track, optionally embeds cover art, and
// writes ID3 metadata.
func processTrack(
	track types.Track,
	cfg settings,
	paths *config.Paths,
	ffmpegPath string,
	progressCh chan<- progressMsg,
) types.DownloadResult {
	query := fmt.Sprintf("ytsearch:%s audio", track.String())
	outputPath := paths.OutputPath(track.String(), string(cfg.format))

	pCh := make(chan downloader.ProgressUpdate, 100)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for pu := range pCh {
			progressCh <- progressMsg{pu}
		}
	}()

	if err := downloader.Download(
		query,
		cfg.quality,
		cfg.format,
		outputPath,
		pCh,
		3,
	); err != nil {
		close(pCh)
		wg.Wait()
		return types.DownloadResult{
			Track:   track,
			Success: false,
			Error:   err.Error(),
		}
	}
	close(pCh)
	wg.Wait()

	finalPath := outputPath + "." + string(cfg.format)
	result := types.DownloadResult{
		Track:      track,
		OutputPath: finalPath,
		Success:    true,
	}

	// Embed cover art unless the user opted out.
	if !cfg.skipCoverArt {
		coverPath, coverErr := downloadCoverArt(track.CoverArtURL, paths.TempDir)
		if coverErr == nil && coverPath != "" {
			if tagErr := tagger.EmbedCoverArt(finalPath, coverPath, ffmpegPath); tagErr != nil {
				result.Error = "cover: " + tagErr.Error()
			}
		}
	}

	// Write ID3 tags.
	if tagErr := tagger.TagMetadata(finalPath, track, ffmpegPath); tagErr != nil {
		if result.Error != "" {
			result.Error += "; tags: " + tagErr.Error()
		} else {
			result.Error = tagErr.Error()
		}
	}

	return result
}

// downloadCoverArt fetches a cover image from url and saves it to tempDir.
// Returns the file path, or an empty string if no URL was given.
func downloadCoverArt(url, tempDir string) (string, error) {
	if url == "" {
		return "", nil
	}

	if err := os.MkdirAll(tempDir, 0o755); err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}

	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".jpg"
	}
	dest := filepath.Join(tempDir, "cover"+ext)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetch cover: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cover HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return "", fmt.Errorf("create cover file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		return "", fmt.Errorf("write cover: %w", err)
	}

	return dest, nil
}

// ── listenDownload ──────────────────────────────────────────────────────

// listenDownload returns a tea.Cmd that blocks until a message arrives on
// one of the download channels. It is called after every download-related
// message to re-arm the listener.
func listenDownload(m *model) tea.Cmd {
	if m.progressCh == nil && m.resultCh == nil && m.totalCh == nil && m.errCh == nil {
		return nil
	}
	return func() tea.Msg {
		// Priority: progress and results first, then total count, then errors.
		select {
		case p, ok := <-m.progressCh:
			if ok {
				return p
			}
			m.progressCh = nil
		case r, ok := <-m.resultCh:
			if ok {
				return r
			}
			m.resultCh = nil
		default:
		}

		select {
		case p, ok := <-m.progressCh:
			if ok {
				return p
			}
			m.progressCh = nil
		case r, ok := <-m.resultCh:
			if ok {
				return r
			}
			m.resultCh = nil
		case t, ok := <-m.totalCh:
			if ok {
				return totalMsg{count: t}
			}
			m.totalCh = nil
		case e := <-m.errCh:
			return errMsg{e}
		}

		return nil
	}
}

// ── View ────────────────────────────────────────────────────────────────

func (m model) View() string {
	style := lipgloss.NewStyle().Width(m.width).Height(m.height)

	switch m.state {
	case stateSettings:
		return style.Render(settingsView(m))
	case stateDownloading:
		return style.Render(progressView(m))
	case stateResults:
		return style.Render(resultsView(m))
	default:
		return ""
	}
}

// ── Key helpers ─────────────────────────────────────────────────────────

func keyMatches(msg tea.KeyMsg, pattern string) bool {
	switch pattern {
	case "enter":
		return msg.Type == tea.KeyEnter
	case "backspace":
		return msg.Type == tea.KeyBackspace
	case "esc":
		return msg.Type == tea.KeyEsc
	case "ctrl+c":
		return msg.Type == tea.KeyCtrlC
	case "q":
		return msg.Type == tea.KeyRunes && string(msg.Runes) == "q"
	default:
		return false
	}
}

func isRuneKey(msg tea.KeyMsg) bool {
	return msg.Type == tea.KeyRunes && len(msg.Runes) > 0 && string(msg.Runes) != "q"
}

func isDigitKey(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes || len(msg.Runes) != 1 {
		return false
	}
	r := msg.Runes[0]
	return r >= '0' && r <= '9'
}

func digitValue(msg tea.KeyMsg) int {
	if !isDigitKey(msg) {
		return -1
	}
	return int(msg.Runes[0] - '0')
}

func isAffirmative(s string) bool {
	switch s {
	case "y", "Y", "yes", "Yes", "YES", "1", "true", "True", "TRUE":
		return true
	default:
		return false
	}
}

// Run starts the Bubble Tea TUI and blocks until the user exits.
func Run() error {
	m := New()
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
