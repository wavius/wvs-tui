package main

import (
	"context"
	"fmt"
	"log"
	"net/url"

	"github.com/chromedp/chromedp"
	"github.com/manifoldco/promptui"

	"main/scraper"
)

var miruro scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "https://www.miruro.to",
	Search: "/search?query=",
	Query:  "",
	Type:   scraper.Anime,

	// Result attributes
	ResultReadySelector: "._styledCardWrapper_eylnt_1",
	ResultSelector:      "._styledCardWrapper_eylnt_1",
	ResultNameSelector:  "title",
	ResultLinkSelector:  "href",

	// Episode attributes
	EpisodeReadySelector: ".player video",
	EpisodeSelector:      "._root_p7i3w_1",
	EpisodeNameSelector:  "title",
}

var next scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "",
	Search: "",
	Query:  "",
	Type:   scraper.Show,

	// Result attributes
	ResultReadySelector: "",
	ResultSelector:      "",
	ResultNameSelector:  "",
	ResultLinkSelector:  "",

	// Episode attributes
	EpisodeReadySelector: "",
	EpisodeSelector:      "",
	EpisodeNameSelector:  "",
}

var Sites = []scraper.SearchAttributes{miruro, next}

func main() {
	bubbletea_main()
}

func promptui_main() {
	clear()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	chromedp.Run(ctx)

	var results []scraper.SearchResult
	var episodes []scraper.EpisodeResult

	searchPrompt := promptui.Prompt{
		Label: "Search:",
		Templates: &promptui.PromptTemplates{
			Prompt:  `{{ "?" | blue }} {{ . | bold }} `,
			Valid:   `{{ "?" | blue }} {{ . | bold }} `,
			Success: `{{ "✔" | green }} {{ . | bold }} `,
		},
	}

	query, err := searchPrompt.Run()
	errorFatal("Search prompt failed", err)

	// Convert raw string to url query
	query = url.QueryEscape(query)

	for _, site := range Sites {
		if site.Site == "" {
			continue
		}
		if site.Type == scraper.Anime {
			if !scraper.FoundAnime(ctx, query) {
				continue
			}
		}

		site.Query = query
		_ = site.SearchForQuery(ctx, &results)
	}

	if len(results) == 0 {
		errorFatal("Search failed", fmt.Errorf("no results found"))
	}

	label := fmt.Sprintf("Found %d results", len(results))

	prompt := promptui.Select{
		Label: label,
		Items: results,
		Templates: &promptui.SelectTemplates{
			Active:   `> {{ .Name | cyan }}`,
			Inactive: `  {{ .Name }}`,
			Selected: `{{ "✔" | green }} {{ .Name | bold }}`,
		},
		Size:     15,
		HideHelp: true,
	}

	showIndex, _, err := prompt.Run()
	errorFatal("Result prompt failed", err)
	selectedResult := results[showIndex]
	err = selectedResult.Source.GetEpisodes(ctx, &episodes, selectedResult)
	errorFatal("Episode fetch failed", err)

	if len(episodes) == 0 {
		episodes = append(episodes, scraper.EpisodeResult{
			Name:          "Movie",
			Number:        1,
			ClickSelector: "",
		})
		label = "Found 1 movie"
	} else {
		label = fmt.Sprintf("Found %d episodes", len(episodes))
	}

	prompt = promptui.Select{
		Label: label,
		Items: episodes,
		Templates: &promptui.SelectTemplates{
			Active:   `> {{ .Name | cyan }}`,
			Inactive: `  {{ .Name }}`,
			Selected: `{{ "✔" | green }} {{ .Name | bold }}`,
		},
		Size:     15,
		HideHelp: true,
	}

	episodeIndex, _, err := prompt.Run()
	errorFatal("Episode prompt failed", err)

	fmt.Println("\nGetting video stream...")
	videoURL, err := selectedResult.Source.GetVideo(ctx, episodes[episodeIndex], selectedResult)
	errorFatal("Failed to get video stream", err)

	fmt.Println("\nPlaying...")
	selectedResult.Source.PlayVideo(videoURL)
	errorFatal("MPV crashed or failed to start", err)
}

func clear() {
	fmt.Print("\033[H\033[2J")
}

func errorFatal(msg string, err error) {
	if err != nil {
		log.Fatalf("%s: %v", msg, err)
	}
}
