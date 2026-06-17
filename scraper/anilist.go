package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

type AniListResponse struct {
	Data struct {
		Media struct {
			Description string `json:"description"`
			CoverImage  struct {
				Large string `json:"large"`
			} `json:"coverImage"`
		} `json:"Media"`
	} `json:"data"`
}

// Queries the AniList GraphQL API to get an anime's description and cover image
func FetchAniListInfo(ctx context.Context, title string) (string, string, error) {
	query := `
	query ($search: String) {
		Media(search: $search, type: ANIME) {
			description
			coverImage {
				large
			}
		}
	}
	`

	payload := map[string]interface{}{
		"query": query,
		"variables": map[string]interface{}{
			"search": title,
		},
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", "", err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://graphql.anilist.co", bytes.NewReader(bodyBytes))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	var apiResp AniListResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return "", "", err
	}

	desc := apiResp.Data.Media.Description

	// Strip html tags
	desc = strings.ReplaceAll(desc, "<br>", "")
	desc = strings.ReplaceAll(desc, "<br/>", "")
	desc = strings.ReplaceAll(desc, "<br />", "")
	desc = strings.ReplaceAll(desc, "<i>", "")
	desc = strings.ReplaceAll(desc, "</i>", "")
	desc = strings.ReplaceAll(desc, "<b>", "")
	desc = strings.ReplaceAll(desc, "</b>", "")
	// Clean up excess whitespace
	desc = strings.TrimSpace(desc)

	return desc, apiResp.Data.Media.CoverImage.Large, nil
}
