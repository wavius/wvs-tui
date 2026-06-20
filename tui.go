package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"github.com/blacktop/go-termimg"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	sites []scraper.SearchAttributes
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

	m.resultList = list.New(items, delegate, m.width/2, m.height)
	m.resultList.Title = title
	m.state = state
	return m
}

func initialModel(ctx context.Context, sites []scraper.SearchAttributes) tuiModel {
	ti := textinput.New()
	ti.Placeholder = "..."
	ti.Focus()
	ti.CharLimit = 150
	ti.Width = 40

	return tuiModel{
		state:       StateSearch,
		searchInput: ti,
		ctx:         ctx,
		sites:       sites,
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
		c := m.selectedResult.Source.PlayVideo(videoURL)
		return m, tea.ExecProcess(c, func(err error) tea.Msg {
			return videoPlaybackFinishedMsg{err}
		})

	case videoPlaybackFinishedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		m.state = StateSelectEpisode
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Prevent nil pointer on initial load
		if m.state == StateSelectResult || m.state == StateSelectEpisode {
			m.resultList.SetSize(msg.Width/2, msg.Height)
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
					return m, searchQueryCmd(m.ctx, m.sites, m.searchInput.Value())
				}
			case StateSelectResult:
				if m.resultList.SelectedItem() != nil {
					m.state = StateLoadingEpisodes
					m.selectedResult = m.resultList.SelectedItem().(scraper.SearchResult)
					return m, episodeQueryCmd(m.ctx, m.selectedResult)
				}
			case StateSelectEpisode:
				if m.resultList.SelectedItem() != nil {
					m.state = StateLoadingVideo
					episode := m.resultList.SelectedItem().(scraper.EpisodeResult)
					return m, videoQueryCmd(m.ctx, episode, m.selectedResult)
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
		return fmt.Sprintf("Error: %v\nPress ESC to quit.\n", m.err)
	}

	switch m.state {
	case StateSearch:
		return fmt.Sprintf(
			"Search:\n\n%s\n",
			m.searchInput.View(),
		)
	case StateLoadingResults:
		return "Loading Results...\n"
	case StateSelectResult:
		listView := m.resultList.View()
		rightPane := ""
		item := m.resultList.SelectedItem()
		if item != nil {
			res := item.(scraper.SearchResult)
			imgView := res.RenderedImg
			descView := res.Desc

			if imgView != "" || descView != "" {
				rightWidth := (m.width / 2) - 10
				if rightWidth < 20 {
					rightWidth = 20
				}
				descStyle := lipgloss.NewStyle().Width(rightWidth).PaddingTop(1)
				descView = descStyle.Render(descView)

				rightPane = lipgloss.JoinVertical(lipgloss.Left, imgView, descView)
			}
		}

		if rightPane != "" {
			listView = lipgloss.NewStyle().Width(m.width / 2).Render(listView)
			rightPane = lipgloss.NewStyle().PaddingLeft(5).Render(rightPane)
			return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, listView, rightPane)
		}
		return "\n" + listView

	case StateSelectEpisode:
		listView := m.resultList.View()
		rightPane := ""

		res := m.selectedResult
		imgView := res.RenderedImg
		descView := res.Desc

		if imgView != "" || descView != "" {
			rightWidth := (m.width / 2) - 10
			if rightWidth < 20 {
				rightWidth = 20
			}
			descStyle := lipgloss.NewStyle().Width(rightWidth).PaddingTop(1)
			descView = descStyle.Render(descView)

			rightPane = lipgloss.JoinVertical(lipgloss.Left, imgView, descView)
		}

		if rightPane != "" {
			listView = lipgloss.NewStyle().Width(m.width / 2).Render(listView)
			rightPane = lipgloss.NewStyle().PaddingLeft(5).Render(rightPane)
			return "\n" + lipgloss.JoinHorizontal(lipgloss.Top, listView, rightPane)
		}
		return "\n" + listView
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

func bubbletea_main() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	chromedp.Run(ctx)

	p := tea.NewProgram(initialModel(ctx, Sites), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
	}
}

// Background commands

type resultSearchFinishedMsg struct {
	results []scraper.SearchResult
	err     error
}

func searchQueryCmd(ctx context.Context, sites []scraper.SearchAttributes, query string) tea.Cmd {
	return func() tea.Msg {
		var results []scraper.SearchResult

		for _, site := range sites {
			if site.Site == "" {
				continue
			}
			if site.Type == scraper.Anime {
				if !scraper.FoundAnime(ctx, query) {
					continue
				}
			}

			site.Query = query
			var siteResults []scraper.SearchResult
			err := site.SearchForQuery(ctx, &siteResults)
			if err == nil {
				results = append(results, siteResults...)
			}
		}

		if len(results) == 0 {
			return resultSearchFinishedMsg{err: fmt.Errorf("no results found")}
		}

		// Update numbering
		for i := range results {
			results[i].Number = i + 1
		}

		var wg sync.WaitGroup
		for i := range results {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				desc, imgURL, _ := scraper.FetchAniListInfo(ctx, results[index].Name)
				if desc != "" {
					results[index].Desc = desc
				}
				if imgURL != "" {
					results[index].ImgURL = imgURL

					// Download and render the image
					req, err := http.NewRequestWithContext(ctx, "GET", imgURL, nil)
					if err == nil {
						resp, err := http.DefaultClient.Do(req)
						if err == nil {
							defer resp.Body.Close()
							img, err := termimg.From(resp.Body)
							if err == nil {
								widget := termimg.NewImageWidget(img)
								widget.SetSize(105, 70).SetProtocol(termimg.Halfblocks)
								rendered, _ := widget.Render()
								results[index].RenderedImg = rendered
							}
						}
					}
				}
			}(i)
		}
		wg.Wait()

		return resultSearchFinishedMsg{results: results, err: nil}
	}
}

type episodeSearchFinishedMsg struct {
	episodes []scraper.EpisodeResult
	err      error
}

func episodeQueryCmd(ctx context.Context, result scraper.SearchResult) tea.Cmd {
	return func() tea.Msg {
		var episodes []scraper.EpisodeResult

		err := result.Source.GetEpisodes(ctx, &episodes, result)

		if len(episodes) == 0 {
			episodes = append(episodes, scraper.EpisodeResult{
				Name:          "Movie",
				Number:        1,
				ClickSelector: "",
			})
		}

		return episodeSearchFinishedMsg{episodes: episodes, err: err}
	}
}

type videoQueryFinishedMsg struct {
	videoURL string
	err      error
}

func videoQueryCmd(ctx context.Context, episode scraper.EpisodeResult, result scraper.SearchResult) tea.Cmd {
	return func() tea.Msg {
		videoURL, err := result.Source.GetVideo(ctx, episode, result)
		return videoQueryFinishedMsg{videoURL: videoURL, err: err}
	}
}

// Emitted when tea.ExecProcess finishes running mpv
type videoPlaybackFinishedMsg struct {
	err error
}
