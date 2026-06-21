package main

import (
	"context"
	"fmt"
	"log"
	"main/scraper"
	"net/url"

	"github.com/chromedp/chromedp"
	"github.com/manifoldco/promptui"
)

// USED FOR DEBUGGING ONLY:
// CLI is currently unused in favor of the BubbleTea TUI implementation

func promptui_main(sites []scraper.SearchAttributes, initialQuery string) {
	clear()

	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("disable-site-isolation-trials", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-software-rasterizer", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

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

	query := initialQuery
	if query == "" {
		var err error
		query, err = searchPrompt.Run()
		errorFatal("Search prompt failed", err)
	}

	// Convert raw string to url query
	query = url.QueryEscape(query)

	for _, site := range sites {
		if site.Site == "" {
			continue
		}

		if !site.IsUp(ctx) {
			continue
		}

		if site.Type == scraper.Anime {
			if !scraper.FoundAnime(ctx, query) {
				continue
			}
		} else if site.Type == scraper.ShowsAndMovies {
			if !scraper.FoundTMDB(ctx, query) {
				continue
			}
		}

		site.Query = query
		err := site.SearchForQuery(ctx, &results)
		if err == nil && len(results) > 0 {
			break
		}
	}

	if len(results) == 0 {
		errorFatal("Search failed", fmt.Errorf("no results found"))
	}

	fmt.Println("\n[DEBUG] --- SEARCH RESULTS ---")
	for i, res := range results {
		fmt.Printf("[%d] %s\n    Link: %s\n", i+1, res.Name, res.Link)
	}
	fmt.Println("------------------------------")

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

	fmt.Printf("\n[DEBUG] You selected:\n    Name: %s\n    Link: %s\n\n", selectedResult.Name, selectedResult.Link)

	var seasons []scraper.SeasonResult
	var selectedSeason *scraper.SeasonResult

	if selectedResult.Source.SeasonContainerSelector != "" {
		err = selectedResult.Source.GetSeasons(ctx, &seasons, selectedResult)
		if err == nil && len(seasons) > 0 {
			label = fmt.Sprintf("Found %d seasons", len(seasons))

			prompt = promptui.Select{
				Label: label,
				Items: seasons,
				Templates: &promptui.SelectTemplates{
					Active:   `> {{ .Name | cyan }}`,
					Inactive: `  {{ .Name }}`,
					Selected: `{{ "✔" | green }} {{ .Name | bold }}`,
				},
				Size:     15,
				HideHelp: true,
			}

			seasonIndex, _, err := prompt.Run()
			errorFatal("Season prompt failed", err)
			selectedSeason = &seasons[seasonIndex]

			fmt.Printf("\n[DEBUG] You selected Season:\n    Name: %s\n\n", selectedSeason.Name)
		}
	}

	err = selectedResult.Source.GetEpisodes(ctx, &episodes, selectedResult, selectedSeason)
	errorFatal("Episode fetch failed", err)

	if len(episodes) == 0 {
		clickSel := ""
		if selectedResult.Source.MovieContainer != "" {
			clickSel = fmt.Sprintf(`document.querySelectorAll("%s")[0].click()`, selectedResult.Source.MovieContainer)
		}

		episodes = append(episodes, scraper.EpisodeResult{
			Name:          "Movie",
			Number:        1,
			ClickSelector: clickSel,
			Container:     selectedResult.Source.MovieContainer,
		})
		label = "Found 1 movie"
	} else {
		label = fmt.Sprintf("Found %d episodes", len(episodes))
	}

	fmt.Println("\n[DEBUG] --- EPISODES ---")
	for i, ep := range episodes {
		fmt.Printf("[%d] %s\n    Selector: %s\n", i+1, ep.Name, ep.ClickSelector)
	}
	fmt.Println("------------------------")

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

	selectedEpisode := episodes[episodeIndex]
	fmt.Printf("\n[DEBUG] You selected Episode:\n    Name: %s\n    Selector: %s\n\n", selectedEpisode.Name, selectedEpisode.ClickSelector)

	fmt.Println("\nGetting video stream...")
	videoURL, err := selectedResult.Source.GetVideo(ctx, selectedEpisode, selectedResult)
	errorFatal("Failed to get video stream", err)

	fmt.Printf("\n[DEBUG] Found Video URL:\n    %s\n\n", videoURL)

	fmt.Println("\nPlaying...")
	cmd := selectedResult.Source.PlayVideo(videoURL)
	err = cmd.Run()
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
