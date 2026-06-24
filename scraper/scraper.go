package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"github.com/blacktop/go-termimg"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type MediaType int

const (
	Anime MediaType = iota
	ShowsAndMovies
	All
)

// Config to navigate and extract video stream from a site
type SiteConfig struct {
	Site   string
	Search string
	Type   MediaType

	ResultReadySelector string // CSS to wait for after searching
	ResultClickSelector string // CSS for the first search result to click
	SeasonSelector      string // CSS for season buttons/dropdown
	EpisodeSelector     string // CSS for episode items
	VideoReadySelector  string // CSS to wait for before extracting video
	MovieSelector       string // CSS for movie play button (no episodes)
}

// TMDB search result
type SearchResult struct {
	Name      string
	Number    int
	Date      string
	Desc      string
	ImgURL    string
	RawImg    *termimg.Image
	TMDBID    int
	MediaType string // "tv" or "movie"
	Site      SiteConfig
}

func (r SearchResult) Title() string { return r.Name }
func (r SearchResult) Description() string {
	if r.Date != "" {
		return r.Date
	}
	return "No date available."
}
func (r SearchResult) FilterValue() string { return r.Name }

// TMDB season
type SeasonResult struct {
	Name         string
	Number       int
	SeasonNumber int
	EpisodeCount int
}

func (s SeasonResult) Title() string       { return s.Name }
func (s SeasonResult) Description() string { return fmt.Sprintf("%d episodes", s.EpisodeCount) }
func (s SeasonResult) FilterValue() string { return s.Name }

// TMDB episode
type EpisodeResult struct {
	Name          string
	Number        int
	EpisodeNumber int
	AirDate       string
	Desc          string
	ImgURL        string
	RawImg        *termimg.Image
}

func (r EpisodeResult) Title() string       { return r.Name }
func (r EpisodeResult) Description() string { return r.AirDate }
func (r EpisodeResult) FilterValue() string { return r.Name }

func (s SiteConfig) IsUp(ctx context.Context) bool {
	timeoutCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(timeoutCtx, "GET", s.Site, nil)
	if err != nil {
		return false
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500
}

// Navigates the streaming site via chromedp and extracts the video stream URL
func (s SiteConfig) GetVideo(ctx context.Context, showName string, seasonNum int, episodeNum int, isMovie bool) (string, error) {
	var streamURL string

	tabCtx, cancelTab := chromedp.NewContext(ctx)
	defer cancelTab()

	chromedp.ListenTarget(tabCtx, func(ev any) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			if strings.Contains(e.Request.URL, ".m3u8") || strings.Contains(e.Request.URL, ".mp4") {
				if streamURL == "" {
					streamURL = e.Request.URL
				}
			}
		}
	})

	searchURL := s.Site + s.Search + url.QueryEscape(showName)

	actions := []chromedp.Action{
		network.Enable(),
		chromedp.Navigate(searchURL),
	}

	// Wait for results and click the first match
	if s.ResultReadySelector != "" {
		actions = append(actions, chromedp.WaitReady(s.ResultReadySelector))
	}
	if s.ResultClickSelector != "" {
		actions = append(actions, chromedp.Click(s.ResultClickSelector, chromedp.ByQuery))
	}

	// Wait for video page to load
	if s.VideoReadySelector != "" {
		actions = append(actions, chromedp.WaitReady(s.VideoReadySelector))
	}

	if isMovie {
		if s.MovieSelector != "" {
			actions = append(actions, chromedp.Click(s.MovieSelector, chromedp.ByQuery))
		}
	} else {
		// Select the correct season
		if s.SeasonSelector != "" && seasonNum > 0 {
			seasonJS := fmt.Sprintf(`
				let items = document.querySelectorAll("%s");
				if (items[%d]) {
					let el = items[%d];
					if (el.tagName && el.tagName.toLowerCase() === 'option') {
						let sel = el.parentElement;
						sel.value = el.value;
						sel.dispatchEvent(new Event('change', { bubbles: true }));
					} else {
						el.click();
					}
				}
			`, s.SeasonSelector, seasonNum-1, seasonNum-1)
			actions = append(actions,
				chromedp.Evaluate(seasonJS, nil),
				chromedp.Sleep(1*time.Second),
			)
		}

		// Click the correct episode
		if s.EpisodeSelector != "" && episodeNum > 0 {
			episodeJS := fmt.Sprintf(`
				let eps = document.querySelectorAll("%s");
				if (eps[%d]) { eps[%d].click(); }
			`, s.EpisodeSelector, episodeNum-1, episodeNum-1)
			actions = append(actions, chromedp.Evaluate(episodeJS, nil))
		}
	}

	actions = append(actions, chromedp.Sleep(3*time.Second))

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 20*time.Second)
	defer cancel()

	if err := chromedp.Run(timeoutCtx, actions...); err != nil {
		return "", fmt.Errorf("failed to extract video stream: %w", err)
	}

	return streamURL, nil
}

func PlayVideo(videoURL string) *exec.Cmd {
	videoURL = strings.ReplaceAll(videoURL, "\\", "/")

	parsed, err := url.Parse(videoURL)
	if err == nil {
		switch {
		case strings.Contains(parsed.Path, "/stream-proxy/pl"):
			if u := parsed.Query().Get("u"); u != "" {
				videoURL = u
			}
			if h := parsed.Query().Get("h"); h != "" {
				var headers map[string]string
				if json.Unmarshal([]byte(h), &headers) == nil {
					var headerArgs []string
					if ref, ok := headers["Referer"]; ok {
						headerArgs = append(headerArgs, "Referer: "+ref)
					}
					if orig, ok := headers["Origin"]; ok {
						headerArgs = append(headerArgs, "Origin: "+orig)
					}
					if len(headerArgs) > 0 {
						headerArg := "--http-header-fields=" + strings.Join(headerArgs, ",")
						return exec.Command("mpv", "--hwdec=auto", "--quiet", "--ytdl-format=bestvideo+bestaudio/best", "--hls-bitrate=max", headerArg, videoURL)
					}
				}
			}

		case strings.Contains(parsed.Path, "/proxy/m3u8"):
			if u := parsed.Query().Get("url"); u != "" {
				videoURL = u
			}
			var headerArgs []string
			if ref := parsed.Query().Get("referer"); ref != "" {
				headerArgs = append(headerArgs, "Referer: "+ref)
			}
			if orig := parsed.Query().Get("origin"); orig != "" {
				headerArgs = append(headerArgs, "Origin: "+orig)
			}
			if len(headerArgs) > 0 {
				headerArg := "--http-header-fields=" + strings.Join(headerArgs, ",")
				return exec.Command("mpv", "--hwdec=auto", "--quiet", "--ytdl-format=bestvideo+bestaudio/best", "--hls-bitrate=max", headerArg, videoURL)
			}
		}
	}

	return exec.Command("mpv", "--hwdec=auto", "--quiet", "--ytdl-format=bestvideo+bestaudio/best", "--hls-bitrate=max", videoURL)
}
