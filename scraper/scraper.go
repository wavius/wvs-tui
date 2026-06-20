package scraper

import (
	"context"
	"encoding/json"
	"fmt"
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

type SearchAttributes struct {
	Site   string
	Search string
	Query  string
	Type   MediaType

	// Search selectors
	ResultReadySelector string // css
	ResultContainer     string // css
	ResultNameClass     string // css class inside container
	ResultNameAttr      string // html attribute
	ResultLinkClass     string // css class inside container
	ResultLinkAttr      string // html attribute

	// Episode selectors
	EpisodeReadySelector string // css
	EpisodeContainer     string // css
	EpisodeNameClass     string // css class inside container
	EpisodeNameAttr      string // html attribute
}

type SearchResult struct {
	Name        string
	Number      int
	Link        string
	Desc        string
	ImgURL      string
	RenderedImg string
	Source      SearchAttributes
}

// list.Item interface for Bubbletea
func (r SearchResult) Title() string { return r.Name }
func (r SearchResult) Description() string {
	if r.Desc != "" {
		return r.Desc
	}
	return "No description available."
}
func (r SearchResult) FilterValue() string { return r.Name }

type EpisodeResult struct {
	Name          string
	Number        int
	Link          string
	ClickSelector string
}

// list.Item interface for Bubbletea
func (r EpisodeResult) Title() string       { return r.Name }
func (r EpisodeResult) Description() string { return r.Link }
func (r EpisodeResult) FilterValue() string { return r.Name }

func (s SearchAttributes) SearchForQuery(ctx context.Context, results *[]SearchResult) error {
	if results == nil {
		return fmt.Errorf("results pointer cannot be nil")
	}

	url := s.Site + s.Search + s.Query

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
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

		item := SearchResult{
			Name:   extractNodeData(timeoutCtx, node, s.ResultNameClass, s.ResultNameAttr),
			Number: i + 1,
			Link:   s.Site + extractNodeData(timeoutCtx, node, s.ResultLinkClass, s.ResultLinkAttr),
			Source: s,
		}

		*results = append(*results, item)

	}

	return nil
}

func (s SearchAttributes) GetEpisodes(ctx context.Context, episodes *[]EpisodeResult, result SearchResult) error {
	if episodes == nil {
		return fmt.Errorf("episodes pointer cannot be nil")
	}

	url := result.Link

	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var nodes []*cdp.Node
	if err := chromedp.Run(
		timeoutCtx,
		chromedp.Navigate(url),
		chromedp.WaitReady(s.EpisodeReadySelector),
		chromedp.Nodes(s.EpisodeContainer, &nodes, chromedp.ByQueryAll, chromedp.AtLeast(0)),
	); err != nil {
		return fmt.Errorf("could not find video: %w", err)
	}

	for i, node := range nodes {

		selector := fmt.Sprintf(`document.querySelectorAll("%s")[%d].click()`, s.EpisodeContainer, i)

		item := EpisodeResult{
			Name:          extractNodeData(timeoutCtx, node, s.EpisodeNameClass, s.EpisodeNameAttr),
			Number:        i + 1,
			ClickSelector: selector,
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
		actions = append(actions,
			chromedp.WaitReady(s.EpisodeContainer),
			chromedp.Evaluate(episode.ClickSelector, nil),
		)
	} else {
		actions = append(actions, chromedp.WaitReady(s.EpisodeReadySelector))
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
			var text string
			err := chromedp.Run(ctx, chromedp.Text(cssClass, &text, chromedp.ByQuery, chromedp.FromNode(parent)))
			if err == nil {
				return strings.TrimSpace(text)
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
					return exec.Command("mpv", "--hwdec=auto", "--profile=fast", "--quiet", headerArg, videoURL)
				}
			}
		}
	}

	// Default run mpv
	return exec.Command("mpv", "--hwdec=auto", "--profile=fast", "--quiet", videoURL)
}
