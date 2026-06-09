package main

import (
	"context"
	"fmt"
	"log"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
)

type searchAttributes struct {
	site           string
	search         string
	query          string
	resultSelector string
	nameSelector   string
	linkSelector   string
}

type searchResult struct {
	name string
	link string
}

func main() {
	var results []searchResult

	attributes := searchAttributes{
		site:           "https://www.miruro.to",
		search:         "/search?query=",
		query:          "Naruto",
		resultSelector: "._styledCardWrapper_eylnt_1",
		nameSelector:   "title",
		linkSelector:   "href",
	}

	url := attributes.site + attributes.search + attributes.query

	ctx, cancel := chromedp.NewContext(context.Background())
	defer cancel()

	var nodes []*cdp.Node
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		// chromedp.Sleep(3 * time.Second),
		chromedp.WaitReady(attributes.resultSelector),
		chromedp.Nodes(attributes.resultSelector, &nodes, chromedp.ByQueryAll),
	); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d links:\n", len(nodes))
	for _, node := range nodes {

		item := searchResult{
			name: node.AttributeValue(attributes.nameSelector),
			link: node.AttributeValue(attributes.linkSelector),
		}

		results = append(results, item)
	}

	for _, result := range results {
		fmt.Printf("Name: %s\n", result.name)
		fmt.Printf("Link: %s\n\n", attributes.site+result.link)
	}
}
