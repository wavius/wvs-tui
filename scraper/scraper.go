package scraper

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

type SearchAttributes struct {
	Site   string
	Search string
	Query  string

	// Search selectors
	ResultSelector     string
	ResultNameSelector string
	ResultLinkSelector string

	// Episode selectors
	EpisodeSelector     string
	EpisodeNameSelector string
	EpisodeLinkSelector string
}

type SearchResult struct {
	Name   string
	Number int
	Link   string
}

type EpisodeResult struct {
	Name   string
	Number int
	Link   string
}

func (s SearchAttributes) SearchForQuery(results *[]SearchResult) error {
	if results == nil {
		return fmt.Errorf("results pointer cannot be nil")
	}

	url := s.Site + s.Search + s.Query

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

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

func (s SearchAttributes) GetEpisodes(episodes *[]EpisodeResult, result SearchResult) error {
	url := result.Link

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

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

		item := EpisodeResult{
			Name:   node.AttributeValue(s.EpisodeNameSelector),
			Number: i + 1,
			Link:   s.Site + node.AttributeValue(s.EpisodeLinkSelector),
		}

		*episodes = append(*episodes, item)
	}
	return nil
}

func (r SearchResult) Print() {
	fmt.Printf("[%d] %s\n", r.Number, r.Name)
}
