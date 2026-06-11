package scraper

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type SearchAttributes struct {
	Site   string
	Search string
	Query  string

	// Search selectors
	ResultSelector     string // css
	ResultNameSelector string // html
	ResultLinkSelector string // html

	// Episode selectors
	EpisodeSelector     string // css
	EpisodeNameSelector string // html
	EpisodeLinkSelector string
}

type SearchResult struct {
	Name   string
	Number int
	Link   string
}

type EpisodeResult struct {
	Name          string
	Number        int
	ClickSelector string
}

func (s SearchAttributes) SearchForQuery(ctx context.Context, results *[]SearchResult) error {
	if results == nil {
		return fmt.Errorf("results pointer cannot be nil")
	}

	url := s.Site + s.Search + s.Query

	var nodes []*cdp.Node
	if err := chromedp.Run(
		ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady(s.ResultSelector),
		chromedp.Nodes(s.ResultSelector, &nodes, chromedp.ByQueryAll),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d:\n", len(nodes))
	for i, node := range nodes {

		item := SearchResult{
			Name:   node.AttributeValue(s.ResultNameSelector),
			Number: i + 1,
			Link:   s.Site + node.AttributeValue(s.ResultLinkSelector),
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

	var nodes []*cdp.Node
	if err := chromedp.Run(
		ctx,
		chromedp.Navigate(url),
		chromedp.WaitReady(s.EpisodeSelector),
		chromedp.Nodes(s.EpisodeSelector, &nodes, chromedp.ByQueryAll),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d episodes:\n", len(nodes))
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

func (s SearchAttributes) GetVideo(ctx context.Context, episode EpisodeResult, result SearchResult) string {
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

	if err := chromedp.Run(
		ctx,
		network.Enable(),
		chromedp.Navigate(url),
		chromedp.WaitReady(s.EpisodeSelector),
		chromedp.Click(episode.ClickSelector, chromedp.ByQuery),
		chromedp.Sleep(3*time.Second),
	); err != nil {
		log.Fatal(err)
	}
	return streamURL
}

func (r SearchResult) Print() {
	fmt.Printf("[%d] %s\n", r.Number, r.Name)
}

func (e EpisodeResult) Print() {
	fmt.Printf("[%d] %s\n", e.Number, e.Name)
}
