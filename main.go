package main

import (
	"main/scraper"
)

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

	// Episode attributes
	EpisodeReadySelector: ".player video",
	EpisodeContainer:     "._root_p7i3w_1",
	EpisodeNameClass:     "",
	EpisodeNameAttr:      "title",
}

var streamvaults scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "https://streamvaults.ru",
	Search: "/search?q=",
	Query:  "",
	Type:   scraper.All,

	// Result attributes
	ResultReadySelector: ".text-3xl",
	ResultContainer:     ".group.block.w-full",
	ResultNameClass:     ".text-sm.font-semibold.text-zinc-300",
	ResultNameAttr:      "text",
	ResultLinkClass:     "",
	ResultLinkAttr:      "href",

	// Episode attributes
	EpisodeReadySelector: ".text-2xl",
	EpisodeContainer:     ".flex.gap-3.p-3.rounded-xl",
	EpisodeNameClass:     ".text-sm.font-semibold.line-clamp-1",
	EpisodeNameAttr:      "text",
}

var Sites = []scraper.SearchAttributes{ /*miruro,*/ streamvaults}

func main() {
	bubbletea_main(Sites)
}
