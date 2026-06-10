package main

import (
	"log"

	"main/scraper"
)

func main() {
	var results []scraper.SearchResult

	attributes := scraper.SearchAttributes{
		Site:   "https://www.miruro.to",
		Search: "/search?query=",
		Query:  "Naruto",

		ResultSelector:     "._styledCardWrapper_eylnt_1",
		ResultNameSelector: "title",
		ResultLinkSelector: "href",

		// You'll need to figure out what these actual selectors are!
		EpisodeSelector:     ".episode-btn-class",
		EpisodeNameSelector: "title",
		EpisodeLinkSelector: "href",
	}

	err := attributes.SearchForQuery(&results)
	if err != nil {
		log.Fatal("Search failed:", err)
	}

	for _, result := range results {
		result.Print()
	}

	// Example of how you would call GetEpisodes for the FIRST result:
	/*
		if len(results) > 0 {
			var episodes []scraper.EpisodeResult
			err = attributes.GetEpisodes(&episodes, results[0])
			if err != nil {
				log.Fatal("Episode fetch failed:", err)
			}
			// print episodes here...
		}
	*/
}
