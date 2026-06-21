package main

import (
	"os"
	"strings"

	"main/scraper"
)

// Unused
var miruro scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "https://www.miruro.to",
	Search: "/search?query=",
	Query:  "",
	Type:   scraper.Anime,

	// Result attributes
	ResultReadySelector: "._styledCardWrapper_eylnt_1",
	ResultContainer:     "._styledCardWrapper_eylnt_1",
	ResultNameClass:     "",
	ResultNameAttr:      "title",
	ResultLinkClass:     "",
	ResultLinkAttr:      "href",
	ResultDateClass:     "",
	ResultDateAttr:      "",

	// Season attributes
	SeasonContainerSelector: "",
	SeasonClickSelector:     "",
	SeasonNameClass:         "",
	SeasonNameAttr:          "",

	// Episode attributes
	EpisodeReadySelector: ".player video",
	EpisodeContainer:     "._root_p7i3w_1",
	EpisodeNameClass:     "",
	EpisodeNameAttr:      "title",

	// Formatting
	EpisodeAddNumbering: false,
}

var streamvaults scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "https://streamvaults.ru",
	Search: "/search?q=",
	Query:  "",
	Type:   scraper.All,

	// Result attributes
	ResultReadySelector: ".text-3xl",
	ResultContainer:     ".group.block.w-full",
	ResultNameClass:     ".text-sm.font-semibold",
	ResultNameAttr:      "text",
	ResultLinkClass:     "",
	ResultLinkAttr:      "href",
	ResultDateClass:     ".bg-transparent",
	ResultDateAttr:      "text",

	// Season attributes
	SeasonContainerSelector: ".ml-auto.bg-zinc-800.border option",
	SeasonClickSelector:     "",
	SeasonNameClass:         "",
	SeasonNameAttr:          "text",

	// Episode attributes
	EpisodeReadySelector: ".text-2xl",
	EpisodeContainer:     ".flex.gap-3.p-3.rounded-xl",
	EpisodeNameClass:     ".text-sm.font-semibold.line-clamp-1",
	EpisodeNameAttr:      "text",

	// Formatting
	EpisodeAddNumbering: true,
}

var gaiaflix scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "https://gaiaflix.live",
	Search: "/search?q=",
	Query:  "",
	Type:   scraper.All,

	// Result attributes
	ResultReadySelector: ".group",
	ResultContainer:     ".group.relative.bg-card.rounded-2xl",
	ResultNameClass:     ".font-bold.truncate.text-sm",
	ResultNameAttr:      "text",
	ResultLinkClass:     "",
	ResultLinkAttr:      "href",
	ResultDateClass:     ".flex.justify-between.items-center",
	ResultDateAttr:      "text",

	// Season attributes
	SeasonContainerSelector: ".px-6.rounded-2xl.font-semibold",
	SeasonClickSelector:     "",
	SeasonNameClass:         "",
	SeasonNameAttr:          "text",

	// Episode attributes
	EpisodeReadySelector: ".bg-background.p-4.rounded-xl",
	EpisodeContainer:     ".bg-background.p-4.rounded-xl",
	EpisodeNameClass:     ".font-semibold.text-sm.truncate.block",
	EpisodeNameAttr:      "text",

	// Formatting
	EpisodeAddNumbering: true,
}

var Sites = []scraper.SearchAttributes{ /*miruro,*/ streamvaults, gaiaflix}

func main() {
	var query string
	if len(os.Args) > 1 {
		query = strings.Join(os.Args[1:], " ")
	}

	// tui
	bubbletea_main(Sites, query)

	// cli (debug)
	// promptui_main(Sites, query)
}
