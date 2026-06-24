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

var TMDBApiKey string // Injected at compile time via ldflags

func getAPIKey() string {
	if TMDBApiKey != "" {
		return TMDBApiKey
	}
	return os.Getenv("TMDB_API_KEY")
}

func tmdbGet(ctx context.Context, path string, target interface{}) error {
	apiKey := getAPIKey()
	if apiKey == "" {
		return fmt.Errorf("TMDB API key not provided")
	}

	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	reqURL := fmt.Sprintf("https://api.themoviedb.org/3%s%sapi_key=%s", path, sep, apiKey)

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("TMDB API returned status: %d", resp.StatusCode)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

// --- Response types ---

type TMDBMultiResult struct {
	ID           int    `json:"id"`
	MediaType    string `json:"media_type"` // "tv" or "movie"
	Name         string `json:"name"`       // TV shows
	Title        string `json:"title"`      // Movies
	Overview     string `json:"overview"`
	PosterPath   string `json:"poster_path"`
	ReleaseDate  string `json:"release_date"`
	FirstAirDate string `json:"first_air_date"`
}

func (r TMDBMultiResult) DisplayName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.Title
}

func (r TMDBMultiResult) DisplayDate() string {
	dateStr := ""
	if r.FirstAirDate != "" {
		dateStr = r.FirstAirDate
	} else if r.ReleaseDate != "" {
		dateStr = r.ReleaseDate
	}

	year := "Unknown Year"
	if len(dateStr) >= 4 {
		year = dateStr[:4]
	} else if dateStr != "" {
		year = dateStr
	}

	mType := "Unknown"
	if r.MediaType == "movie" {
		mType = "Film"
	} else if r.MediaType == "tv" {
		mType = "Show"
	}

	return fmt.Sprintf("%s • %s", year, mType)
}

func (r TMDBMultiResult) PosterURL() string {
	if r.PosterPath != "" {
		return "https://image.tmdb.org/t/p/w500" + r.PosterPath
	}
	return ""
}

type TMDBSeason struct {
	Name         string `json:"name"`
	SeasonNumber int    `json:"season_number"`
	EpisodeCount int    `json:"episode_count"`
}

type TMDBEpisode struct {
	Name          string `json:"name"`
	EpisodeNumber int    `json:"episode_number"`
	AirDate       string `json:"air_date"`
	Overview      string `json:"overview"`
	StillPath     string `json:"still_path"`
}

func (e TMDBEpisode) StillURL() string {
	if e.StillPath != "" {
		return "https://image.tmdb.org/t/p/w500" + e.StillPath
	}
	return ""
}

// --- API functions ---

func SearchTMDB(ctx context.Context, query string) ([]TMDBMultiResult, error) {
	var resp struct {
		Results []TMDBMultiResult `json:"results"`
	}

	path := fmt.Sprintf("/search/multi?query=%s", url.QueryEscape(query))
	if err := tmdbGet(ctx, path, &resp); err != nil {
		return nil, err
	}

	// Filter to only tv and movie results
	var filtered []TMDBMultiResult
	for _, r := range resp.Results {
		switch r.MediaType {
		case "tv", "movie":
			filtered = append(filtered, r)
		}
	}

	return filtered, nil
}

func GetTMDBSeasons(ctx context.Context, showID int) ([]TMDBSeason, error) {
	var resp struct {
		Seasons []TMDBSeason `json:"seasons"`
	}

	path := fmt.Sprintf("/tv/%d", showID)
	if err := tmdbGet(ctx, path, &resp); err != nil {
		return nil, err
	}

	// Filter out "Specials" (season 0) if there are regular seasons
	var filtered []TMDBSeason
	for _, s := range resp.Seasons {
		if s.SeasonNumber > 0 {
			filtered = append(filtered, s)
		}
	}

	if len(filtered) == 0 {
		return resp.Seasons, nil
	}
	return filtered, nil
}

func GetTMDBEpisodes(ctx context.Context, showID int, seasonNumber int) ([]TMDBEpisode, error) {
	var resp struct {
		Episodes []TMDBEpisode `json:"episodes"`
	}

	path := fmt.Sprintf("/tv/%d/season/%d", showID, seasonNumber)
	if err := tmdbGet(ctx, path, &resp); err != nil {
		return nil, err
	}

	return resp.Episodes, nil
}
