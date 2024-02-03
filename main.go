package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"
)

const root_url string = "https://www.questdiagnostics.com/healthcare-professionals/test-directory"

func main() {
	ctx, notifyCancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer notifyCancel()

	ctx, cdpCancel := chromedp.NewContext(ctx)
	defer cdpCancel()

	if err := chromedp.Run(ctx, scrape()); err != nil {
		log.Fatalln(err)
	}
}

func scrape() chromedp.Tasks {
	var nodes []*cdp.Node
	var specialtyUrls []string
	var resultsNodes []*cdp.Node

	return chromedp.Tasks{
		chromedp.Navigate(root_url),
		chromedp.Sleep(3 * time.Second),
		chromedp.WaitReady("#a2zContainer"),
		chromedp.Nodes("#a2zContainer > div.component-body-area > div.a2z-area > ul > li", &nodes),
		chromedp.ActionFunc(crawlChildren(&nodes)),
		//chromedp.ActionFunc(displayNodes(&nodes)),
		chromedp.ActionFunc(gatherSpecialtyUrls(&nodes, &specialtyUrls)),
		chromedp.ActionFunc(gatherTestPageUrls(&specialtyUrls, &resultsNodes)),
	}
}

func crawlChildren(nodes *[]*cdp.Node) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var retErr error
		// depth -1 for the entire subtree
		// do your best to limit the size of the subtree
		for _, node := range *nodes {
			if err := dom.RequestChildNodes(node.NodeID).WithDepth(-1).Do(ctx); err != nil {
				retErr = errors.Join(retErr, err)
			}
		}
		return retErr
	}
}

func gatherSpecialtyUrls(nodes *[]*cdp.Node, specialtyUrls *[]string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		for _, node := range *nodes {
			for _, child := range node.Children {
				*specialtyUrls = append(*specialtyUrls, child.AttributeValue("href"))
			}
		}
		log.Printf("%d specialty urls", len(*specialtyUrls))
		return nil
	}
}

func displayNodes(nodes *[]*cdp.Node) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		printNodes(os.Stdout, *nodes, "", "  ")
		return nil
	}
}

func printNodes(w io.Writer, nodes []*cdp.Node, padding, indent string) {
	// This will block until the chromedp listener closes the channel
	for _, node := range nodes {
		switch {
		case node.NodeName == "#text":
			fmt.Fprintf(w, "%s#text: %q\n", padding, node.NodeValue)
		default:
			fmt.Fprintf(w, "%s%s:\n", padding, strings.ToLower(node.NodeName))
			if n := len(node.Attributes); n > 0 {
				fmt.Fprintf(w, "%sattributes:\n", padding+indent)
				for i := 0; i < n; i += 2 {
					fmt.Fprintf(w, "%s%s: %q\n", padding+indent+indent, node.Attributes[i], node.Attributes[i+1])
				}
			}
		}
		if node.ChildNodeCount > 0 {
			fmt.Fprintf(w, "%schildren:\n", padding+indent)
			printNodes(w, node.Children, padding+indent+indent, indent)
		}
	}
}

func gatherTestPageUrls(specialtyUrls *[]string, nodes *[]*cdp.Node) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var retErr error
		var tmpNodes []*cdp.Node
		for _, specialtyUrl := range *specialtyUrls {
			log.Printf("Visiting %s", specialtyUrl)
			retErr = errors.Join(retErr, chromedp.Run(
				ctx,
				chromedp.Navigate(fmt.Sprintf("%s&rows=4000", specialtyUrl)),
				chromedp.Sleep(10*time.Second),
				chromedp.Nodes("md-card", &tmpNodes),
			))
			*nodes = append(*nodes, tmpNodes...)
			log.Printf("number of md-cards: %d", len(*nodes))
		}
		return retErr
	}
}
