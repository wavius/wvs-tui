package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type TMDBResponse struct {
	Results []struct {
		Overview   string `json:"overview"`
		PosterPath string `json:"poster_path"`
	} `json:"results"`
}

var TMDBApiKey string // Can be injected at compile time via ldflags

func FoundTMDB(ctx context.Context, title string) bool {
	apiKey := TMDBApiKey
	if apiKey == "" {
		apiKey = os.Getenv("TMDB_API_KEY")
	}
	
	if apiKey == "" {
		return false
	}

	searchURL := fmt.Sprintf("https://api.themoviedb.org/3/search/multi?api_key=%s&query=%s", apiKey, url.QueryEscape(title))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return false
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	var apiResp TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return false
	}

	return len(apiResp.Results) > 0
}

// Queries the TMDB API to get a media's description and cover image
func FetchTMDBInfo(ctx context.Context, title string) (string, string, error) {
	apiKey := TMDBApiKey
	if apiKey == "" {
		apiKey = os.Getenv("TMDB_API_KEY")
	}

	if apiKey == "" {
		return "", "", fmt.Errorf("TMDB API key not provided at compile time or in environment")
	}

	searchURL := fmt.Sprintf("https://api.themoviedb.org/3/search/multi?api_key=%s&query=%s", apiKey, url.QueryEscape(title))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return "", "", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("TMDB API returned status: %d", resp.StatusCode)
	}

	var apiResp TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", "", err
	}

	if len(apiResp.Results) == 0 {
		return "", "", fmt.Errorf("no results found")
	}

	desc := apiResp.Results[0].Overview
	var imgURL string
	if apiResp.Results[0].PosterPath != "" {
		imgURL = "https://image.tmdb.org/t/p/w500" + apiResp.Results[0].PosterPath
	}

	// Clean up excess whitespace
	desc = strings.TrimSpace(desc)

	return desc, imgURL, nil
}
