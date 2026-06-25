package main

import (
	"context"
	"fmt"

	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/chromedp/chromedp"

	"main/scraper"
)

type AppState int

const (
	StateSearch AppState = iota
	StateLoadingResults
	StateSelectResult
	StateLoadingSeasons
	StateSelectSeason
	StateLoadingEpisodes
	StateSelectEpisode
	StateLoadingVideo
	StatePlayingVideo
)

// Image rendering constants
const (
	// 2:3 aspect ratio (posters)
	PosterAspectW = 2.0
	PosterAspectH = 3.0

	// 16:9 aspect ratio (episode thumbnails)
	StillAspectW = 16.0
	StillAspectH = 9.0

	// Size multiplier (1.0 = fill available space)
	ImageScale = 1.5
)

type tuiModel struct {
	ctx   context.Context
	sites []scraper.SiteConfig
	flags map[string]string
	state AppState
	err   error

	width       int
	height      int
	searchInput textinput.Model
	resultList  list.Model

	selectedResult  scraper.SearchResult
	results         []scraper.SearchResult
	selectedSeason  scraper.SeasonResult
	seasons         []scraper.SeasonResult
	selectedEpisode scraper.EpisodeResult
	episodes        []scraper.EpisodeResult

	cachedImgString string
	cachedImgName   string
	cachedImgWidth  int
	cachedImgHeight int
}

func (m tuiModel) loadList(items []list.Item, title string, state AppState, showDesc bool) tuiModel {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = showDesc

	w := m.width / 2
	if w <= 0 {
		w = 40
	}
	h := m.height
	if h <= 0 {
		h = 24
	}

	m.resultList = list.New(items, delegate, w, h)
	m.resultList.Title = title
	m.state = state
	return m
}

func initialModel(ctx context.Context, sites []scraper.SiteConfig, flags map[string]string, initialQuery string) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "..."
	ti.Focus()
	ti.CharLimit = 150
	ti.Width = 40

	state := StateSearch
	if initialQuery != "" {
		ti.SetValue(initialQuery)
		state = StateLoadingResults
	}

	return tuiModel{
		state:       state,
		searchInput: ti,
		ctx:         ctx,
		sites:       sites,
		flags:       flags,
	}
}

func (m tuiModel) Init() tea.Cmd {
	if m.state == StateLoadingResults {
		return tea.Batch(textinput.Blink, searchQueryCmd(m.ctx, m.searchInput.Value()))
	}
	return textinput.Blink
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case resultSearchFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.results = msg.results
		m = m.loadList(toListItems(msg.results), "Select Result", StateSelectResult, true)
		m.updateImageCache()
		return m, nil

	case seasonSearchFinishedMsg:

		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if len(msg.seasons) == 0 {
			m.state = StateLoadingVideo
			return m, videoQueryCmd(m.ctx, m.sites, m.selectedResult.TMDBID, 0, 0, true)
		}
		m.seasons = msg.seasons
		m = m.loadList(toListItems(msg.seasons), "Select Season", StateSelectSeason, true)
		return m, nil

	case episodeSearchFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.episodes = msg.episodes
		m = m.loadList(toListItems(msg.episodes), "Select Episode", StateSelectEpisode, true)
		m.updateImageCache()
		return m, nil

	case videoQueryFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.state = StatePlayingVideo
		c := scraper.PlayVideo(msg.siteName, msg.videoURL, msg.subtitles, msg.headers, m.flags["q"])
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return videoPlaybackFinishedMsg{err}
		})

	case videoPlaybackFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		if m.selectedResult.MediaType == "movie" {
			m.state = StateSelectResult
		} else {
			m.state = StateSelectEpisode
		}
		// Force image cache update after state change
		m.cachedImgName = ""
		m.updateImageCache()
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		switch m.state {
		case StateSelectResult, StateSelectSeason, StateSelectEpisode, StatePlayingVideo:
			m.resultList.SetSize(msg.Width/2, msg.Height)
		}

	case tea.KeyMsg:

		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":

			return m.handleEnter()
		case "backspace":
			m = m.handleBackspace()
		}
	}

	switch m.state {
	case StateSearch:
		m.searchInput, cmd = m.searchInput.Update(msg)
	case StateSelectResult, StateSelectSeason, StateSelectEpisode:
		m.resultList, cmd = m.resultList.Update(msg)
	}

	m.updateImageCache()
	return m, cmd
}

func (m tuiModel) handleEnter() (tea.Model, tea.Cmd) {

	switch m.state {
	case StateSearch:
		if m.searchInput.Value() != "" {
			m.state = StateLoadingResults
			return m, searchQueryCmd(m.ctx, m.searchInput.Value())
		}
	case StateSelectResult:
		item := m.resultList.SelectedItem()

		if item == nil {
			break
		}
		m.selectedResult = item.(scraper.SearchResult)

		switch m.selectedResult.MediaType {
		case "tv":
			m.state = StateLoadingSeasons
			return m, seasonQueryCmd(m.ctx, m.selectedResult.TMDBID)
		case "movie":
			m.state = StateLoadingVideo
			return m, videoQueryCmd(m.ctx, m.sites, m.selectedResult.TMDBID, 0, 0, true)
		}
	case StateSelectSeason:
		if m.resultList.SelectedItem() == nil {
			break
		}
		m.selectedSeason = m.resultList.SelectedItem().(scraper.SeasonResult)
		m.state = StateLoadingEpisodes
		return m, episodeQueryCmd(m.ctx, m.selectedResult.TMDBID, m.selectedSeason.SeasonNumber)
	case StateSelectEpisode:
		if m.resultList.SelectedItem() == nil {
			break
		}
		m.selectedEpisode = m.resultList.SelectedItem().(scraper.EpisodeResult)
		m.state = StateLoadingVideo
		return m, videoQueryCmd(m.ctx, m.sites, m.selectedResult.TMDBID, m.selectedSeason.SeasonNumber, m.selectedEpisode.EpisodeNumber, false)
	case StatePlayingVideo:
		return m, nil
	}
	return m, nil
}

func (m tuiModel) handleBackspace() tuiModel {
	switch m.state {
	case StateSelectResult:
		m.searchInput.SetValue("")
		m.state = StateSearch
	case StateSelectSeason:
		m = m.loadList(toListItems(m.results), "Select Result", StateSelectResult, true)
	case StateSelectEpisode:
		if len(m.seasons) > 0 {
			m = m.loadList(toListItems(m.seasons), "Select Season", StateSelectSeason, true)
		} else {
			m = m.loadList(toListItems(m.results), "Select Result", StateSelectResult, true)
		}
	}
	return m
}

func (m *tuiModel) updateImageCache() {
	// Set image width to remaining terminal width, and height to half terminal height
	// Esnures description text is always visible and not pushed offscreen
	rightWidth := m.width - (m.width / 2) - 4
	w := rightWidth
	h := m.height / 2
	if h <= 0 {
		h = 10
	}
	if w <= 0 {
		w = 20
	}

	var rawImg *termimg.Image
	var cacheKey string
	isEpisode := false

	switch m.state {
	case StateSelectResult:
		if item := m.resultList.SelectedItem(); item != nil {
			res := item.(scraper.SearchResult)
			rawImg = res.RawImg
			cacheKey = res.Name + res.MediaType // ensure uniqueness
		}
	case StateSelectSeason:
		rawImg = m.selectedResult.RawImg
		cacheKey = m.selectedResult.Name
	case StateSelectEpisode:
		isEpisode = true
		if item := m.resultList.SelectedItem(); item != nil {
			ep := item.(scraper.EpisodeResult)
			rawImg = ep.RawImg
			cacheKey = ep.Name + ep.AirDate
			// Fallback to show poster if episode thumbnail is missing
			if rawImg == nil && m.selectedResult.RawImg != nil {
				rawImg = m.selectedResult.RawImg
				cacheKey = m.selectedResult.Name
				isEpisode = false // Render it as a poster
			}
		}
	}

	if rawImg == nil {
		m.cachedImgString = ""
		m.cachedImgName = cacheKey
		m.cachedImgWidth = rightWidth
		m.cachedImgHeight = 0
		return
	}

	// Skip expensive rendering if the image is already cached for this item and size
	if m.cachedImgName == cacheKey && m.cachedImgWidth == rightWidth && m.cachedImgString != "" {
		return
	}

	var aspectW, aspectH float64
	if isEpisode {
		aspectW, aspectH = StillAspectW, StillAspectH
	} else {
		aspectW, aspectH = PosterAspectW, PosterAspectH
	}

	// Calculate target w/h ratio in terminal cells
	ratio := aspectH / (2.0 * aspectW)

	// Available space adjusted by scale multiplier
	availableW := float64(rightWidth) * ImageScale
	availableH := float64(m.height-8) * ImageScale

	// Start with available width
	w = int(availableW)
	h = int(float64(w) * ratio)

	// If height is too big for available space, scale based on height
	if float64(h) > availableH {
		h = int(availableH)
		w = int(float64(h) / ratio)
	}

	if h < 10 {
		h = 10
	}
	if w < 10 {
		w = 10
	}

	// termimg preserves native aspect ratio within this bounding box
	widget := termimg.NewImageWidget(rawImg)
	widget.SetSize(w, h).SetProtocol(termimg.Halfblocks)
	rendered, _ := widget.Render()

	m.cachedImgString = rendered
	m.cachedImgName = cacheKey
	m.cachedImgWidth = rightWidth
	m.cachedImgHeight = strings.Count(rendered, "\n") + 1
}

func (m tuiModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\nPress ESC to quit.\n", m.err)
	}

	switch m.state {
	case StateSearch:
		return fmt.Sprintf("Search:\n\n%s\n", m.searchInput.View())
	case StateLoadingResults:
		return "Loading Results...\n"
	case StateSelectResult:
		res := m.currentResult()
		return m.renderDetailView(res.Desc)
	case StateLoadingSeasons:
		return "Loading Seasons...\n"
	case StateSelectSeason:
		return m.renderDetailView("▶ YOU ARE NOW SELECTING A SEASON ◀\n\n" + m.selectedResult.Desc)
	case StateSelectEpisode:
		desc := ""
		if item := m.resultList.SelectedItem(); item != nil {
			desc = item.(scraper.EpisodeResult).Desc
		}
		return m.renderDetailView(desc)
	case StateLoadingEpisodes:
		return "Loading Episodes...\n"
	case StateLoadingVideo:
		return "Loading Video...\n"
	case StatePlayingVideo:
		return "Playing Video...\n"
	default:
		return "Unknown State\n"
	}
}

func (m tuiModel) currentResult() scraper.SearchResult {
	if item := m.resultList.SelectedItem(); item != nil {
		return item.(scraper.SearchResult)
	}
	return scraper.SearchResult{}
}

func (m tuiModel) renderDetailView(desc string) string {
	leftPaneWidth := m.width / 2
	listView := m.resultList.View()

	imgView := m.cachedImgString
	if imgView != "" {
		imgView = lipgloss.NewStyle().PaddingTop(1).Render(imgView)
	}

	if imgView == "" && desc == "" {
		return "\n" + listView
	}

	rightWidth := m.width - leftPaneWidth - 10
	if rightWidth < 20 {
		rightWidth = 20
	}

	maxDescHeight := m.height - m.cachedImgHeight - 6
	if maxDescHeight < 1 {
		maxDescHeight = 1
	}

	descView := lipgloss.NewStyle().Width(rightWidth).MaxHeight(maxDescHeight).PaddingTop(1).Render(desc)
	rightPane := lipgloss.JoinVertical(lipgloss.Left, imgView, descView)

	listView = lipgloss.NewStyle().Width(leftPaneWidth).Render(listView)
	rightPane = lipgloss.NewStyle().PaddingLeft(5).Render(rightPane)
	return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, listView, rightPane)
}

func bubbletea_main(sites []scraper.SiteConfig, flags map[string]string, initialQuery string) {

	headless := true
	if flags["h"] != "" {
		fmt.Printf("Usage: wvs [query] [flags]\n\n")
		fmt.Printf("Commands:\n")
		fmt.Printf("  wvs              Launch interactive search mode\n")
		fmt.Printf("  wvs <query>      Direct search for a specific show or movie\n\n")
		fmt.Printf("Flags:\n")
		fmt.Printf("  -h, -help        List all commands and flags\n")
		fmt.Printf("  -l, -list        List available sources and their status\n")
		fmt.Printf("  -d, -debug       Disable headless browser mode\n")
		fmt.Printf("  -s, -source      Select a specific source by number or name (default is fastest source)\n")
		fmt.Printf("  -q, -quality     Set video quality (default is 1080p)\n")
		return
	}

	if flags["l"] != "" {
		ctx := context.Background()
		for i, s := range sites {
			status := "DOWN"
			if s.IsUp(ctx) {
				status = "UP"
			}
			fmt.Printf("%d: [%s] %s\n", i+1, status, s.Name)
		}
		return
	}

	if flags["d"] != "" {
		headless = false
	}

	if source, ok := flags["s"]; ok {
		if num, err := strconv.Atoi(source); err == nil {
			if num > 0 && num <= len(sites) {
				sites = []scraper.SiteConfig{sites[num-1]}
			} else {
				fmt.Printf("Invalid source number: %d\n", num)
				return
			}
		} else {
			found := false
			for _, s := range sites {
				if s.Name == source {
					sites = []scraper.SiteConfig{s}
					found = true
					break
				}
			}
			if !found {
				fmt.Printf("Invalid source name: %s\n", source)
				return
			}
		}
	}

	// run TUI
	cwd, _ := os.Getwd()
	uBlockPath := filepath.Join(cwd, "extensions", "uBOL")

	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", headless),
		chromedp.Flag("disable-site-isolation-trials", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-software-rasterizer", true),
		chromedp.Flag("autoplay-policy", "no-user-gesture-required"),
		chromedp.Flag("disable-extensions-except", uBlockPath),
		chromedp.Flag("load-extension", uBlockPath),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	p := tea.NewProgram(initialModel(ctx, sites, flags, initialQuery), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}

// Background commands

type resultSearchFinishedMsg struct {
	results []scraper.SearchResult
	err     error
}

type seasonSearchFinishedMsg struct {
	seasons []scraper.SeasonResult
	err     error
}

type episodeSearchFinishedMsg struct {
	episodes []scraper.EpisodeResult
	err      error
}

type videoQueryFinishedMsg struct {
	siteName  string
	videoURL  string
	subtitles []string
	headers   map[string]string
	err       error
}

type videoPlaybackFinishedMsg struct {
	err error
}

func toListItems[T list.Item](items []T) []list.Item {
	result := make([]list.Item, len(items))
	for i, item := range items {
		result[i] = item
	}
	return result
}

func searchQueryCmd(ctx context.Context, query string) tea.Cmd {
	return func() tea.Msg {
		// Search TMDB
		tmdbResults, err := scraper.SearchTMDB(ctx, query)
		if err != nil || len(tmdbResults) == 0 {
			return resultSearchFinishedMsg{err: fmt.Errorf("no results found")}
		}

		var results []scraper.SearchResult
		for i, r := range tmdbResults {
			results = append(results, scraper.SearchResult{
				Name:      r.DisplayName(),
				Number:    i + 1,
				Date:      r.DisplayDate(),
				Desc:      r.Overview,
				ImgURL:    r.PosterURL(),
				TMDBID:    r.ID,
				MediaType: r.MediaType,
			})
		}

		// Fetch poster images in parallel
		var wg sync.WaitGroup
		for i := range results {
			if results[i].ImgURL == "" {
				continue
			}
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				req, err := http.NewRequestWithContext(ctx, "GET", results[idx].ImgURL, nil)
				if err != nil {
					return
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return
				}
				defer resp.Body.Close()
				if img, err := termimg.From(resp.Body); err == nil {
					results[idx].RawImg = img
				}
			}(i)
		}
		wg.Wait()

		return resultSearchFinishedMsg{results: results}
	}
}

func seasonQueryCmd(ctx context.Context, tmdbID int) tea.Cmd {
	return func() tea.Msg {
		tmdbSeasons, err := scraper.GetTMDBSeasons(ctx, tmdbID)
		if err != nil {
			return seasonSearchFinishedMsg{err: err}
		}

		var seasons []scraper.SeasonResult
		for i, s := range tmdbSeasons {
			seasons = append(seasons, scraper.SeasonResult{
				Name:         s.Name,
				Number:       i + 1,
				SeasonNumber: s.SeasonNumber,
				EpisodeCount: s.EpisodeCount,
			})
		}

		return seasonSearchFinishedMsg{seasons: seasons}
	}
}

func episodeQueryCmd(ctx context.Context, tmdbID int, seasonNumber int) tea.Cmd {
	return func() tea.Msg {
		tmdbEpisodes, err := scraper.GetTMDBEpisodes(ctx, tmdbID, seasonNumber)
		if err != nil {
			return episodeSearchFinishedMsg{err: err}
		}

		var episodes []scraper.EpisodeResult
		for i, e := range tmdbEpisodes {
			episodes = append(episodes, scraper.EpisodeResult{
				Name:          fmt.Sprintf("%d - %s", e.EpisodeNumber, e.Name),
				Number:        i + 1,
				EpisodeNumber: e.EpisodeNumber,
				AirDate:       e.AirDate,
				Desc:          e.Overview,
				ImgURL:        e.StillURL(),
			})
		}

		// Fetch episode stills in parallel
		var wg sync.WaitGroup
		for i := range episodes {
			if episodes[i].ImgURL == "" {
				continue
			}
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				req, err := http.NewRequestWithContext(ctx, "GET", episodes[idx].ImgURL, nil)
				if err != nil {
					return
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					return
				}
				defer resp.Body.Close()
				if img, err := termimg.From(resp.Body); err == nil {
					episodes[idx].RawImg = img
				}
			}(i)
		}
		wg.Wait()

		return episodeSearchFinishedMsg{episodes: episodes}
	}
}

func videoQueryCmd(ctx context.Context, sites []scraper.SiteConfig, tmdbID int, seasonNum int, episodeNum int, isMovie bool) tea.Cmd {
	return func() tea.Msg {

		// race all sites to find video
		// use context to cancel all queries when one succeeds
		raceCtx, cancel := context.WithCancel(ctx)
		defer cancel()

		type result struct {
			siteName  string
			videoURL  string
			subtitles []string
			headers   map[string]string
			err       error
		}

		resultChannel := make(chan result, len(sites))

		for _, site := range sites {
			// query every site at the same time using goroutine
			go func(s scraper.SiteConfig) {
				videoURL, subtitles, headers, err := s.GetVideo(raceCtx, tmdbID, seasonNum, episodeNum, isMovie)
				resultChannel <- result{s.Name, videoURL, subtitles, headers, err}
			}(site)
		}

		var lastErr error
		for range sites {
			res := <-resultChannel
			if res.err == nil && res.videoURL != "" {
				return videoQueryFinishedMsg{siteName: res.siteName, videoURL: res.videoURL, subtitles: res.subtitles, headers: res.headers, err: nil}
			}
			lastErr = res.err
		}

		return videoQueryFinishedMsg{err: fmt.Errorf("all sites failed to find video: %v", lastErr)}
	}
}
