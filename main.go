package main

import (
	"os"
	"strings"

	"main/scraper"
)

var flickystream = scraper.SiteConfig{
	Name:               "flickystream",
	Site:               "https://flickystream.su",
	Type:               scraper.All,
	MovieURLTemplate:   "%s/player/movie/%d",
	TVURLTemplate:      "%s/player/tv/%d/%d/%d",
	VideoReadySelector: "video",
	PlayButtonSelector: "button[aria-label='Play']",
}

var gaiaflix = scraper.SiteConfig{
	Name:               "gaiaflix",
	Site:               "https://gaiaflix.live",
	Type:               scraper.All,
	MovieURLTemplate:   "%s/watch/%d?type=movie",
	TVURLTemplate:      "%s/watch/%d?type=tv&s=%d&e=%d",
	VideoReadySelector: "iframe",
	PlayButtonSelector: "button.absolute.top-1\\/2",
}

var Sites = []scraper.SiteConfig{flickystream, gaiaflix}

func main() {
	// disable termimg's CSI queries to prevent it from reading os.Stdin
	// this would annoyingly swallow key presses
	os.Setenv("TERMIMG_BYPASS_DETECTION", "halfblocks")

	var queryParts []string
	var flags []string

	if len(os.Args) > 1 {
		supportedFlags := map[string]string{
			"-h":     "-h",
			"--help": "-h",
			"-help":  "-h",
			"-s":     "-s",
		}

		for _, arg := range os.Args[1:] {
			if mappedFlag, exists := supportedFlags[arg]; exists {
				flags = append(flags, mappedFlag)
			} else {
				queryParts = append(queryParts, arg)
			}
		}
	}

	query := strings.Join(queryParts, " ")
	bubbletea_main(Sites, flags, query)
}
