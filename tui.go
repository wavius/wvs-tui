package main

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/chromedp/chromedp"

	"main/scraper"
)

// Application states
type AppState int

const (
	StateSearch AppState = iota
	StateLoadingResults
	StateSelectResult
	StateLoadingEpisodes
	StateSelectEpisode
	StateLoadingVideo
	StatePlayingVideo
)

type tuiModel struct {
	// Core state
	ctx   context.Context
	site  scraper.SearchAttributes
	state AppState
	err   error

	// Terminal & UI
	width       int
	height      int
	searchInput textinput.Model
	resultList  list.Model

	// Cached data
	selectedResult scraper.SearchResult
	results        []scraper.SearchResult
	episodes       []scraper.EpisodeResult
}

// Helper to load items into list
func (m tuiModel) loadList(items []list.Item, title string, state AppState) tuiModel {
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	m.resultList = list.New(items, delegate, m.width, m.height)
	m.resultList.Title = title
	m.state = state
	return m
}

func initialModel(ctx context.Context, site scraper.SearchAttributes) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "..."
	ti.Focus()
	ti.CharLimit = 150
	ti.Width = 40

	return tuiModel{
		state:       StateSearch,
		searchInput: ti,
		ctx:         ctx,
		site:        site,
	}
}

func (m tuiModel) Init() tea.Cmd {
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
		var items []list.Item
		for _, res := range msg.results {
			items = append(items, res)
		}

		return m.loadList(items, "Select Result", StateSelectResult), nil

	case episodeSearchFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		m.episodes = msg.episodes
		var items []list.Item
		for _, eps := range msg.episodes {
			items = append(items, eps)
		}

		return m.loadList(items, "Select Episode", StateSelectEpisode), nil

	case videoQueryFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		videoURL := msg.videoURL
		m.state = StatePlayingVideo

		// Suspend the TUI while video plays, restore it when done
		c := m.site.PlayVideo(videoURL)
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return videoPlaybackFinishedMsg{err}
		})

	case videoPlaybackFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		// Reload episodes after mpv closes
		var items []list.Item
		for _, eps := range m.episodes {
			items = append(items, eps)
		}

		return m.loadList(items, "Select Episode", StateSelectEpisode), nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Prevent nil pointer on initial load
		if m.state == StateSelectResult {
			m.resultList.SetSize(msg.Width, msg.Height)
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			switch m.state {
			case StateSearch:
				if m.searchInput.Value() != "" {
					m.state = StateLoadingResults
					return m, searchQueryCmd(m.ctx, m.site, m.searchInput.Value())
				}
			case StateSelectResult:
				if m.resultList.SelectedItem() != nil {
					m.state = StateLoadingEpisodes
					m.selectedResult = m.resultList.SelectedItem().(scraper.SearchResult)
					return m, episodeQueryCmd(m.ctx, m.site, m.selectedResult)
				}
			case StateSelectEpisode:
				if m.resultList.SelectedItem() != nil {
					m.state = StateLoadingVideo
					episode := m.resultList.SelectedItem().(scraper.EpisodeResult)
					return m, videoQueryCmd(m.ctx, m.site, episode, m.selectedResult)
				}
			case StatePlayingVideo:
				return m, nil
			default:
				return m, tea.Quit
			}
		case "backspace":
			switch m.state {
			case StateSelectResult:
				// Clear search bar
				m.searchInput.SetValue("")
				m.state = StateSearch
			case StateSelectEpisode:
				// Reload results
				var items []list.Item
				for _, res := range m.results {
					items = append(items, res)
				}

				m = m.loadList(items, "Select Result", StateSelectResult)
			}
		}
	}

	switch m.state {
	case StateSearch:
		m.searchInput, cmd = m.searchInput.Update(msg)
	case StateSelectResult, StateSelectEpisode:
		m.resultList, cmd = m.resultList.Update(msg)
	}

	return m, cmd
}

func (m tuiModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press ESC to quit.", m.err)
	}

	switch m.state {
	case StateSearch:
		return fmt.Sprintf(
			"Search:\n\n%s\n",
			m.searchInput.View(),
		)
	case StateLoadingResults:
		return "Loading Results...\n"
	case StateSelectResult, StateSelectEpisode:
		return "\n" + m.resultList.View()
	case StateLoadingEpisodes:
		return "Loading Episodes...\n"
	case StateLoadingVideo:
		return "Loading Video...\n"
	case StatePlayingVideo:
		return "Playing Video...\n"
	default:
		return "Unknown State"
	}
}

func bubbletea_main() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	chromedp.Run(ctx)

	p := tea.NewProgram(initialModel(ctx, miruro), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}

// Background commands

type resultSearchFinishedMsg struct {
	results []scraper.SearchResult
	err     error
}

func searchQueryCmd(ctx context.Context, site scraper.SearchAttributes, query string) tea.Cmd {
	return func() tea.Msg {
		var results []scraper.SearchResult

		site.Query = query
		err := site.SearchForQuery(ctx, &results)

		return resultSearchFinishedMsg{results: results, err: err}
	}
}

type episodeSearchFinishedMsg struct {
	episodes []scraper.EpisodeResult
	err      error
}

func episodeQueryCmd(ctx context.Context, site scraper.SearchAttributes, result scraper.SearchResult) tea.Cmd {
	return func() tea.Msg {
		var episodes []scraper.EpisodeResult

		err := site.GetEpisodes(ctx, &episodes, result)

		return episodeSearchFinishedMsg{episodes: episodes, err: err}
	}
}

type videoQueryFinishedMsg struct {
	videoURL string
	err      error
}

func videoQueryCmd(ctx context.Context, site scraper.SearchAttributes, episode scraper.EpisodeResult, result scraper.SearchResult) tea.Cmd {
	return func() tea.Msg {
		videoURL, err := site.GetVideo(ctx, episode, result)
		return videoQueryFinishedMsg{videoURL: videoURL, err: err}
	}
}

// Emitted when tea.ExecProcess finishes running mpv
type videoPlaybackFinishedMsg struct {
	err error
}
