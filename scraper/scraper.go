package scraper

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type MediaType int

const (
	Anime MediaType = iota
	Show
)

type SearchAttributes struct {
	Site   string
	Search string
	Query  string
	Type   MediaType

	// Search selectors
	ResultReadySelector string // css
	ResultSelector      string // css
	ResultNameSelector  string // html
	ResultLinkSelector  string // html

	// Episode selectors
	EpisodeReadySelector string // css
	EpisodeSelector      string // css
	EpisodeNameSelector  string // html
}

type SearchResult struct {
	Name        string
	Number      int
	Link        string
	Desc        string
	ImgURL      string
	RenderedImg string
	Source      SearchAttributes
}

// list.Item interface for Bubbletea
func (r SearchResult) Title() string       { return r.Name }
func (r SearchResult) Description() string {
	if r.Desc != "" {
		return r.Desc
	}
	return "No description available."
}
func (r SearchResult) FilterValue() string { return r.Name }

type EpisodeResult struct {
	Name          string
	Number        int
	Link          string
	isM3u8        bool
	isPopulated   bool
	isDownloading bool
	ClickSelector string
}

// list.Item interface for Bubbletea
func (r EpisodeResult) Title() string       { return r.Name }
func (r EpisodeResult) Description() string { return r.Link }
func (r EpisodeResult) FilterValue() string { return r.Name }

func (s SearchAttributes) SearchForQuery(ctx context.Context, results *[]SearchResult) error {
	if results == nil {
		return fmt.Errorf("results pointer cannot be nil")
	}

	url := s.Site + s.Search + s.Query

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var nodes []*cdp.Node
	if err := chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady(s.ResultReadySelector),
		chromedp.Nodes(s.ResultSelector, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return fmt.Errorf("no results found: %w", err)
	}

	for i, node := range nodes {

		item := SearchResult{
			Name:   node.AttributeValue(s.ResultNameSelector),
			Number: i + 1,
			Link:   s.Site + node.AttributeValue(s.ResultLinkSelector),
			Source: s,
		}

		*results = append(*results, item)

	}

	return nil
}

func (s SearchAttributes) GetEpisodes(ctx context.Context, episodes *[]EpisodeResult, result SearchResult) error {
	if episodes == nil {
		return fmt.Errorf("episodes pointer cannot be nil")
	}

	url := result.Link

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var nodes []*cdp.Node
	if err := chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady(s.EpisodeReadySelector),
		chromedp.Nodes(s.EpisodeSelector, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return fmt.Errorf("could not find video: %w", err)
	}

	for i, node := range nodes {

		selector := fmt.Sprintf("%s:nth-child(%d)", s.EpisodeSelector, i+1)

		item := EpisodeResult{
			Name:          node.AttributeValue(s.EpisodeNameSelector),
			Number:        i + 1,
			ClickSelector: selector,
		}

		*episodes = append(*episodes, item)
	}
	return nil
}

func (s SearchAttributes) GetVideo(ctx context.Context, episode EpisodeResult, result SearchResult) (string, error) {
	url := result.Link
	var streamURL string

	// Subscribe to network events
	chromedp.ListenTarget(ctx, func(ev any) {
		switch e := ev.(type) {
		// Check if the event type contains ".m3u8" or ".mp4"
		case *network.EventRequestWillBeSent:
			if strings.Contains(e.Request.URL, ".m3u8") || strings.Contains(e.Request.URL, ".mp4") {
				streamURL = e.Request.URL
			}
		}
	})

	actions := []chromedp.Action{
		network.Enable(),
		chromedp.Navigate(url),
	}

	if episode.ClickSelector != "" {
		actions = append(actions,
			chromedp.WaitReady(s.EpisodeSelector),
			chromedp.Click(episode.ClickSelector, chromedp.ByQuery),
		)
	} else {
		actions = append(actions, chromedp.WaitReady(s.EpisodeReadySelector))
	}

	actions = append(actions, chromedp.Sleep(3*time.Second))

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	if err := chromedp.Run(timeoutCtx, actions...); err != nil {
		return "", fmt.Errorf("failed to extract video stream: %w", err)
	}
	return streamURL, nil
}

func (r SearchResult) Print() {
	fmt.Printf("[%d] %s\n", r.Number, r.Name)
}

func (e EpisodeResult) Print() {
	fmt.Printf("[%d] %s\n", e.Number, e.Name)
}

func (s SearchAttributes) PlayVideo(videoURL string) *exec.Cmd {
	// Run mpv
	return exec.Command("mpv", "--hwdec=auto", "--profile=fast", "--quiet", videoURL)
}
