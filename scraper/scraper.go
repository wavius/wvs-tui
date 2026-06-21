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

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type MediaType int

const (
	Anime MediaType = iota
	ShowsAndMovies
	All
)

// SearchAttributes defines scraping CSS/HTML selectors:
//
//		ReadySelector: CSS to wait for before scraping.
//
//		Container: CSS for list elements.
//		- "" SeasonContainerSelector skips season selection.
//
//		Class / ClickSelector: CSS for a child inside the container.
//		- "" targets the container itself.
//		- "" MovieContainer defaults to EpisodeContainer
//
//		Attr: HTML attribute to extract (e.g., "href").
//		- "text" extracts inner text
//
//	 AddNumbering: If true, adds numbering to the result name.
//	  - Some websites do it automatically

type SearchAttributes struct {
	Site   string
	Search string
	Query  string
	Type   MediaType

	// Search selectors
	ResultReadySelector string // css
	ResultContainer     string // css
	ResultNameClass     string // css
	ResultNameAttr      string // html attr
	ResultLinkClass     string // css
	ResultLinkAttr      string // html attr
	ResultDateClass     string // css
	ResultDateAttr      string // html attr

	// Season selectors
	SeasonContainerSelector string // css
	SeasonClickSelector     string // css
	SeasonNameClass         string // css
	SeasonNameAttr          string // html attr

	// Episode selectors
	MediaReadySelector string // css
	EpisodeContainer   string // css
	EpisodeNameClass   string // css
	EpisodeNameAttr    string // html attr
	MovieContainer     string // css

	// Formatting
	EpisodeAddNumbering bool
}

type SearchResult struct {
	Name        string
	Number      int
	Date        string
	Link        string
	Desc        string
	ImgURL      string
	RenderedImg string
	Source      SearchAttributes
}

// list.Item interface for Bubbletea
func (r SearchResult) Title() string { return r.Name }
func (r SearchResult) Description() string {
	if r.Date != "" {
		return r.Date
	}
	return "No date available."
}
func (r SearchResult) FilterValue() string { return r.Name }

type SeasonResult struct {
	Name          string
	Number        int
	ClickSelector string
}

// list.Item interface for Bubbletea
func (s SeasonResult) Title() string       { return s.Name }
func (s SeasonResult) Description() string { return "" }
func (s SeasonResult) FilterValue() string { return s.Name }

type EpisodeResult struct {
	Name          string
	Number        int
	Link          string
	ClickSelector string
	Container     string
}

// list.Item interface for Bubbletea
func (r EpisodeResult) Title() string       { return r.Name }
func (r EpisodeResult) Description() string { return r.Link }
func (r EpisodeResult) FilterValue() string { return r.Name }

func (s SearchAttributes) IsUp(ctx context.Context) bool {
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

	// 5xx errors usually mean the server/host is physically down or offline (Cloudflare 521/522)
	return resp.StatusCode < 500
}

func (s SearchAttributes) SearchForQuery(ctx context.Context, results *[]SearchResult) error {
	if results == nil {
		return fmt.Errorf("results pointer cannot be nil")
	}

	url := s.Site + s.Search + s.Query

	tabCtx, cancelTab := chromedp.NewContext(ctx)
	defer cancelTab()

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancel()

	var nodes []*cdp.Node
	if err := chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady(s.ResultReadySelector),
		chromedp.Nodes(s.ResultContainer, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return fmt.Errorf("no results found: %w", err)
	}

	for i, node := range nodes {
		date := ""
		if s.ResultDateClass != "" {
			date = extractNodeData(timeoutCtx, node, s.ResultDateClass, s.ResultDateAttr)
		}

		// if episode link is absolute, make it relative
		rawLink := extractNodeData(timeoutCtx, node, s.ResultLinkClass, s.ResultLinkAttr)
		rawLink = strings.TrimPrefix(rawLink, s.Site)

		item := SearchResult{
			Name:   extractNodeData(timeoutCtx, node, s.ResultNameClass, s.ResultNameAttr),
			Number: i + 1,
			Date:   date,
			Link:   s.Site + rawLink,
			Source: s,
		}

		*results = append(*results, item)

	}

	return nil
}

func (s SearchAttributes) GetSeasons(ctx context.Context, seasons *[]SeasonResult, result SearchResult) error {
	if seasons == nil {
		return fmt.Errorf("seasons pointer cannot be nil")
	}

	url := result.Link

	tabCtx, cancelTab := chromedp.NewContext(ctx)
	defer cancelTab()

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancel()

	var nodes []*cdp.Node
	if err := chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady(s.MediaReadySelector),
		chromedp.Nodes(s.SeasonContainerSelector, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return fmt.Errorf("could not find seasons: %w", err)
	}

	for i, node := range nodes {
		selector := fmt.Sprintf(`
			let el = document.querySelectorAll("%s")[%d];
			if (el && el.tagName && el.tagName.toLowerCase() === 'option') {
				let sel = el.parentElement;
				sel.value = el.value;
				sel.dispatchEvent(new Event('change', { bubbles: true }));
			} else if (el) {
				el.click();
			}
		`, s.SeasonContainerSelector, i)

		item := SeasonResult{
			Name:          extractNodeData(timeoutCtx, node, s.SeasonNameClass, s.SeasonNameAttr),
			Number:        i + 1,
			ClickSelector: selector,
		}

		*seasons = append(*seasons, item)
	}
	return nil
}

func (s SearchAttributes) GetEpisodes(ctx context.Context, episodes *[]EpisodeResult, result SearchResult, season *SeasonResult) error {
	if episodes == nil {
		return fmt.Errorf("episodes pointer cannot be nil")
	}

	url := result.Link

	tabCtx, cancelTab := chromedp.NewContext(ctx)
	defer cancelTab()

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancel()

	var actions []chromedp.Action
	actions = append(actions, chromedp.Navigate(url))

	if season != nil && season.ClickSelector != "" {
		actions = append(actions,
			chromedp.WaitReady(s.SeasonContainerSelector),
			chromedp.Evaluate(season.ClickSelector, nil),
			chromedp.Sleep(1*time.Second), // wait for episodes to load after clicking season
		)
	}

	containerSel := s.EpisodeContainer
	if season == nil && s.MovieContainer != "" {
		containerSel = s.MovieContainer
	}

	var nodes []*cdp.Node
	actions = append(actions,
		chromedp.WaitReady(s.MediaReadySelector),
		chromedp.Nodes(containerSel, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	)

	if err := chromedp.Run(timeoutCtx, actions...); err != nil {
		// If it times out waiting for episodes, we safely ignore the error.
		// This allows the caller (cli/tui) to fall back to the Movie flow!
		return nil
	}

	for i, node := range nodes {

		name := extractNodeData(timeoutCtx, node, s.EpisodeNameClass, s.EpisodeNameAttr)

		if season == nil && s.MovieContainer != "" {
			name = "Movie"
		} else if s.EpisodeAddNumbering {
			name = fmt.Sprintf("%d - %s", i+1, name)
		}

		selector := fmt.Sprintf(`document.querySelectorAll("%s")[%d].click()`, containerSel, i)

		item := EpisodeResult{
			Name:          name,
			Number:        i + 1,
			ClickSelector: selector,
			Container:     containerSel,
		}

		*episodes = append(*episodes, item)
	}
	return nil
}

func (s SearchAttributes) GetVideo(ctx context.Context, episode EpisodeResult, result SearchResult) (string, error) {
	url := result.Link
	var streamURL string

	tabCtx, cancelTab := chromedp.NewContext(ctx)
	defer cancelTab()

	// Subscribe to network events
	chromedp.ListenTarget(tabCtx, func(ev any) {
		switch e := ev.(type) {
		// Check if the event type contains ".m3u8" or ".mp4"
		case *network.EventRequestWillBeSent:
			if strings.Contains(e.Request.URL, ".m3u8") || strings.Contains(e.Request.URL, ".mp4") {
				streamURL = e.Request.URL
			}
		}
	})

	actions := []chromedp.Action{
		network.Enable(),
		chromedp.Navigate(url),
	}

	if episode.ClickSelector != "" {
		waitSel := s.MediaReadySelector
		if episode.Container != "" {
			waitSel = episode.Container
		}
		actions = append(actions,
			chromedp.WaitReady(waitSel),
			chromedp.Evaluate(episode.ClickSelector, nil),
		)
	} else {
		actions = append(actions, chromedp.WaitReady(s.MediaReadySelector))
	}

	actions = append(actions, chromedp.Sleep(3*time.Second))

	timeoutCtx, cancel := context.WithTimeout(tabCtx, 15*time.Second)
	defer cancel()

	if err := chromedp.Run(timeoutCtx, actions...); err != nil {
		return "", fmt.Errorf("failed to extract video stream: %w", err)
	}
	return streamURL, nil
}

func (r SearchResult) Print() {
	fmt.Printf("[%d] %s\n", r.Number, r.Name)
}

func (e EpisodeResult) Print() {
	fmt.Printf("[%d] %s\n", e.Number, e.Name)
}

func textContent(n *cdp.Node) string {
	if n.NodeType == cdp.NodeTypeText {
		return n.NodeValue
	}
	var s strings.Builder
	for _, c := range n.Children {
		s.WriteString(textContent(c))
	}
	return s.String()
}

func extractNodeData(ctx context.Context, parent *cdp.Node, cssClass string, attr string) string {
	if attr == "text" {
		if cssClass != "" {
			var found []*cdp.Node
			err := chromedp.Run(ctx, chromedp.Nodes(cssClass, &found, chromedp.ByQuery, chromedp.AtLeast(0), chromedp.FromNode(parent)))
			if err == nil && len(found) > 0 {
				var text string
				err = chromedp.Run(ctx, chromedp.Text(cssClass, &text, chromedp.ByQuery, chromedp.FromNode(parent)))
				if err == nil {
					return strings.TrimSpace(text)
				}
			}
		}
		return strings.TrimSpace(textContent(parent))
	}

	target := parent
	if cssClass != "" {
		var found []*cdp.Node
		err := chromedp.Run(ctx, chromedp.Nodes(cssClass, &found, chromedp.ByQuery, chromedp.AtLeast(0), chromedp.FromNode(parent)))
		if err == nil && len(found) > 0 {
			target = found[0]
		}
	}

	return target.AttributeValue(attr)
}

func (s SearchAttributes) PlayVideo(videoURL string) *exec.Cmd {
	// Unwrap url if it is behind a stream-proxy
	parsed, err := url.Parse(videoURL)
	if err == nil && strings.Contains(parsed.Path, "/stream-proxy/pl") {
		u := parsed.Query().Get("u")
		if u != "" {
			videoURL = u
		}

		h := parsed.Query().Get("h")
		if h != "" {
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
	}

	// Default run mpv
	return exec.Command("mpv", "--hwdec=auto", "--quiet", "--ytdl-format=bestvideo+bestaudio/best", "--hls-bitrate=max", videoURL)
}
