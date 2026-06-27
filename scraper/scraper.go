package scraper

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"sync"
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

type SiteConfig struct {
	Name             string
	Site             string
	Type             MediaType
	MovieURLTemplate string
	TVURLTemplate    string

	VideoReadySelector string
	PlayButtonSelector string
}

type SearchResult struct {
	Name      string
	Number    int
	Date      string
	Desc      string
	ImgURL    string
	RawImg    *termimg.Image
	TMDBID    int
	MediaType string
}

func (r SearchResult) Title() string { return r.Name }
func (r SearchResult) Description() string {
	if r.Date != "" {
		return r.Date
	}
	return "No date available."
}
func (r SearchResult) FilterValue() string { return r.Name }

type SeasonResult struct {
	Name         string
	Number       int
	SeasonNumber int
	EpisodeCount int
}

func (s SeasonResult) Title() string       { return s.Name }
func (s SeasonResult) Description() string { return fmt.Sprintf("%d episodes", s.EpisodeCount) }
func (s SeasonResult) FilterValue() string { return s.Name }

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

func (s SiteConfig) GetVideo(ctx context.Context, tmdbID, seasonNum, episodeNum int, isMovie bool) (string, []string, map[string]string, error) {
	var streamURL string
	var subtitles []string
	var subtitlesMu sync.Mutex
	headers := make(map[string]string)

	tabCtx, cancelTab := chromedp.NewContext(ctx)
	defer cancelTab()

	vttRegex := regexp.MustCompile(`https?://[^\s"'<>]+\.(?:vtt|srt)`)

	chromedp.ListenTarget(tabCtx, func(ev any) {
		if e, ok := ev.(*network.EventRequestWillBeSent); ok {
			u := e.Request.URL

			// Capture subtitle files
			if strings.Contains(u, ".vtt") || strings.Contains(u, ".srt") {
				subtitlesMu.Lock()
				found := false
				for _, sub := range subtitles {
					if sub == u {
						found = true
						break
					}
				}
				if !found {
					subtitles = append(subtitles, u)
				}
				subtitlesMu.Unlock()
			}

			if streamURL == "" && (strings.Contains(u, ".m3u8") || strings.Contains(u, ".mp4")) {
				streamURL = u
				for k, v := range e.Request.Headers {
					if str, ok := v.(string); ok {
						headers[k] = str
					}
				}
				if headers["Referer"] == "" && headers["referer"] == "" {
					headers["Referer"] = e.DocumentURL
				}
			}
		} else if e, ok := ev.(*network.EventResponseReceived); ok {
			if strings.Contains(e.Response.MimeType, "application/json") {
				reqID := e.RequestID
				go func() {
					var body []byte
					err := chromedp.Run(tabCtx, chromedp.ActionFunc(func(ctx context.Context) error {
						b, err := network.GetResponseBody(reqID).Do(ctx)
						if err != nil {
							return err
						}
						body = b
						return nil
					}))

					if err == nil && len(body) > 0 {
						matches := vttRegex.FindAllString(string(body), -1)
						if len(matches) > 0 {
							subtitlesMu.Lock()
							for _, match := range matches {
								cleanMatch := strings.ReplaceAll(match, "\\/", "/")
								found := false
								for _, sub := range subtitles {
									if sub == cleanMatch {
										found = true
										break
									}
								}
								if !found {
									subtitles = append(subtitles, cleanMatch)
								}
							}
							subtitlesMu.Unlock()
						}
					}
				}()
			}
		}
	})

	var playerURL string
	if isMovie {
		if s.MovieURLTemplate != "" {
			playerURL = fmt.Sprintf(s.MovieURLTemplate, s.Site, tmdbID)
		} else {
			playerURL = fmt.Sprintf("%s/player/movie/%d", s.Site, tmdbID)
		}
	} else {
		if s.TVURLTemplate != "" {
			playerURL = fmt.Sprintf(s.TVURLTemplate, s.Site, tmdbID, seasonNum, episodeNum)
		} else {
			playerURL = fmt.Sprintf("%s/player/tv/%d/%d/%d", s.Site, tmdbID, seasonNum, episodeNum)
		}
	}

	actions := []chromedp.Action{
		network.Enable(),
		chromedp.Navigate(playerURL),
	}

	if s.VideoReadySelector != "" {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			readyCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_ = chromedp.Run(readyCtx, chromedp.WaitReady(s.VideoReadySelector, chromedp.ByQuery))
			return nil
		}))
	}
	if s.PlayButtonSelector != "" {
		actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
			clickCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			err := chromedp.Run(clickCtx,
				chromedp.WaitReady(s.PlayButtonSelector, chromedp.ByQuery),
				chromedp.Click(s.PlayButtonSelector, chromedp.ByQuery),
			)
			if err != nil {
				// Click center of screen to trigger autoplay if button fails
				_ = chromedp.Run(ctx, chromedp.MouseClickXY(400, 300))
			}
			return nil
		}))
	}

	actions = append(actions, chromedp.ActionFunc(func(ctx context.Context) error {
		for range 30 { // 30 seconds
			if streamURL != "" {
				return nil
			}
			time.Sleep(1 * time.Second)
		}
		return nil
	}))

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 30*time.Second)
	defer cancel()

	if err := chromedp.Run(timeoutCtx, actions...); err != nil {
		return "", nil, nil, fmt.Errorf("failed to extract video stream: %w", err)
	}
	if streamURL == "" {
		return "", nil, nil, fmt.Errorf("failed to extract video stream: timeout reached")
	}

	return streamURL, subtitles, headers, nil
}

func PlayVideo(siteName string, videoURL string, subtitles []string, reqHeaders map[string]string, quality string) *exec.Cmd {
	videoURL = strings.ReplaceAll(videoURL, "\\", "/")

	ytdlFormat := "bestvideo+bestaudio/best"
	if quality != "" {
		qualityNum := strings.TrimSuffix(quality, "p")
		ytdlFormat = fmt.Sprintf("bestvideo[height<=?%s]+bestaudio/best[height<=?%s]", qualityNum, qualityNum)
	}

	mpvArgs := []string{"--hwdec=auto", "--quiet", "--script-opts=ytdl_hook-ytdl_path=yt-dlp", "--ytdl-format=" + ytdlFormat, "--hls-bitrate=max"}

	for _, sub := range subtitles {
		mpvArgs = append(mpvArgs, fmt.Sprintf("--sub-file=%s", sub))
	}

	hasUserAgent := false
	for k, v := range reqHeaders {
		lowerK := strings.ToLower(k)
		switch lowerK {
		case "referer":
			mpvArgs = append(mpvArgs, fmt.Sprintf("--referrer=%s", v))
			mpvArgs = append(mpvArgs, fmt.Sprintf("--ytdl-raw-options-append=referer=%s", v))
		case "user-agent":
			hasUserAgent = true
			v = strings.ReplaceAll(v, "HeadlessChrome", "Chrome")
			mpvArgs = append(mpvArgs, fmt.Sprintf("--user-agent=%s", v))
			mpvArgs = append(mpvArgs, fmt.Sprintf("--ytdl-raw-options-append=user-agent=%s", v))
		case "origin":
			mpvArgs = append(mpvArgs, fmt.Sprintf("--http-header-fields-append=Origin: %s", v))
		}
	}

	if !hasUserAgent {
		ua := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
		mpvArgs = append(mpvArgs, fmt.Sprintf("--user-agent=%s", ua))
		mpvArgs = append(mpvArgs, fmt.Sprintf("--ytdl-raw-options-append=user-agent=%s", ua))
	}

	mpvArgs = append(mpvArgs, fmt.Sprintf("--term-playing-msg=Found stream on %s.", siteName))
	mpvArgs = append(mpvArgs, videoURL)

	return exec.Command("mpv", mpvArgs...)
}
