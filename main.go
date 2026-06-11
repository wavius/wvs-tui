package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/chromedp"

	"main/scraper"
)

var miruro scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "https://www.miruro.to",
	Search: "/search?query=",
	Query:  "Naruto",

	// Result attributes
	ResultSelector:     "._styledCardWrapper_eylnt_1",
	ResultNameSelector: "title",
	ResultLinkSelector: "href",

	// Episode attributes
	EpisodeSelector:     "._root_p7i3w_1",
	EpisodeNameSelector: "title",
	EpisodeLinkSelector: "href",
}

func main() {
	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var results []scraper.SearchResult

	err := miruro.SearchForQuery(ctx, &results)
	if err != nil {
		log.Fatal("Search failed:", err)
	}

	for _, result := range results {
		result.Print()
	}

	var episodes []scraper.EpisodeResult
	err = miruro.GetEpisodes(ctx, &episodes, results[0])
	if err != nil {
		log.Fatal("Episode fetch failed:", err)
	}

	for i := 0; i < 3 && i < len(episodes); i++ {
		episodes[i].Print()
	}

	if len(episodes) > 0 {
		fmt.Println("\nSniffing network for video stream...")
		videoURL := miruro.GetVideo(ctx, episodes[0], results[0])
		fmt.Printf("GOT IT: %s\n", videoURL)
	}
}
