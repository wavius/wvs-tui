package main

import (
	"os"
	"strings"

	"main/scraper"
)

var streamvaults = scraper.SiteConfig{
	Site:   "https://streamvaults.ru",
	Search: "/search?q=",
	Type:   scraper.All,

	ResultReadySelector: "",
	ResultClickSelector: "",
	SeasonSelector:      "",
	EpisodeSelector:     "",
	VideoReadySelector:  "",
	MovieSelector:       "",
}

var allyoucanwatch = scraper.SiteConfig{
	Site:   "https://allyoucanwatch.net",
	Search: "/search?q=",
	Type:   scraper.All,

	ResultReadySelector: "",
	ResultClickSelector: "",
	SeasonSelector:      "",
	EpisodeSelector:     "",
	VideoReadySelector:  "",
	MovieSelector:       "",
}

var gaiaflix = scraper.SiteConfig{
	Site:   "https://gaiaflix.live",
	Search: "/search?q=",
	Type:   scraper.All,

	ResultReadySelector: "",
	ResultClickSelector: "",
	SeasonSelector:      "",
	EpisodeSelector:     "",
	VideoReadySelector:  "",
	MovieSelector:       "",
}

var Sites = []scraper.SiteConfig{streamvaults, allyoucanwatch, gaiaflix}

func main() {
	var query string
	if len(os.Args) > 1 {
		query = strings.Join(os.Args[1:], " ")
	}

	bubbletea_main(Sites, query)
}
