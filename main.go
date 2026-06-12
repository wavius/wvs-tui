package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/chromedp"
	"github.com/manifoldco/promptui"

	"main/scraper"
)

var miruro scraper.SearchAttributes = scraper.SearchAttributes{
	Site:   "https://www.miruro.to",
	Search: "/search?query=",
	Query:  "Naruto",

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

func main() {
	clear()

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var results []scraper.SearchResult
	var episodes []scraper.EpisodeResult
	err := miruro.SearchForQuery(ctx, &results)
	if err != nil {
		log.Fatal("Search failed:", err)
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
	if err != nil {
		log.Fatal("Prompt failed:", err)
	}
	err = miruro.GetEpisodes(ctx, &episodes, results[showIndex])
	if err != nil {
		log.Fatal("Episode fetch failed:", err)
	}

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
	if err != nil {
		log.Fatal("Prompt failed:", err)
	}

	fmt.Println("\nGetting video stream...")
	videoURL := miruro.GetVideo(ctx, episodes[episodeIndex], results[showIndex])

	fmt.Println("\nPlaying...")
	err = miruro.PlayVideo(videoURL)
	if err != nil {
		log.Fatal("MPV crashed or failed to start:", err)
	}
}

func clear() {
	fmt.Print("\033[H\033[2J")
}
