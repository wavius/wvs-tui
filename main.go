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

var streamgoblin = scraper.SiteConfig{
	Name:               "streamgoblin",
	Site:               "https://streamgoblin.com",
	Type:               scraper.All,
	MovieURLTemplate:   "%s/player/movie/%d",
	TVURLTemplate:      "%s/player/tv/%d/season/%d/episode/%d",
	VideoReadySelector: "iframe",
	PlayButtonSelector: "",
}

var Sites = []scraper.SiteConfig{streamgoblin, flickystream}

func main() {
	// disable termimg's CSI queries to prevent it from reading os.Stdin
	// this would annoyingly swallow key presses
	os.Setenv("TERMIMG_BYPASS_DETECTION", "halfblocks")

	var queryParts []string
	flags := make(map[string]string)

	if len(os.Args) > 1 {
		args := os.Args[1:]
		for i := 0; i < len(args); i++ {
			arg := args[i]
			switch arg {
			case "-h", "-help":
				flags["h"] = "true"
			case "-l", "-list":
				flags["l"] = "true"
			case "-d", "-debug":
				flags["d"] = "true"
			case "-s", "-source":
				if i+1 < len(args) {
					flags["s"] = args[i+1]
					i++
				}
			case "-q", "-quality":
				if i+1 < len(args) {
					flags["q"] = args[i+1]
					i++
				}
			default:
				queryParts = append(queryParts, arg)
			}
		}
	}

	query := strings.Join(queryParts, " ")
	bubbleteaMain(Sites, flags, query)
}
