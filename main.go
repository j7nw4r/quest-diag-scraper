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
	"strconv"
	"strings"
	"time"
)

const root_url string = "https://www.questdiagnostics.com/healthcare-professionals/test-directory"

type TestData struct {
	name string
}

func main() {
	ctx, notifyCancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer notifyCancel()

	ctx, cdpCancel := chromedp.NewContext(ctx)
	defer cdpCancel()

	var buf []byte
	if err := chromedp.Run(ctx, scrape(&buf)); err != nil {
		log.Fatalln(err)
	}

	if buf == nil || len(buf) == 0 {
		return
	}

	if err := os.WriteFile("fullScreenshot.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}
}

func scrape(buf *[]byte) chromedp.Tasks {
	var nodes []*cdp.Node
	var specialtyUrls []string
	var resultsMap = make(map[int]*cdp.Node)
	var testData = make(map[int]TestData)

	return chromedp.Tasks{
		chromedp.Navigate(root_url),
		chromedp.Sleep(3 * time.Second),
		chromedp.WaitReady("#a2zContainer"),
		chromedp.Nodes("#a2zContainer > div.component-body-area > div.a2z-area > ul > li", &nodes),
		chromedp.ActionFunc(crawlChildren(&nodes)),
		chromedp.ActionFunc(gatherSpecialtyUrls(&nodes, &specialtyUrls)),
		chromedp.ActionFunc(gatherTestPageUrls(&specialtyUrls, &resultsMap)),
		chromedp.ActionFunc(func(ctx context.Context) error {
			for id := range resultsMap {
				if err := chromedp.Run(ctx,
					chromedp.Navigate(fmt.Sprintf("https://testdirectory.questdiagnostics.com/test/test-detail/%d/complement-component-c3c?p=r&q=*&cc=MASTER", id)),
					chromedp.Sleep(2*time.Second),
					chromedp.FullScreenshot(buf, 100),
					chromedp.ActionFunc(gatherTestData(id, &testData)),
				); err != nil {
					log.Printf("could not visit: %d", id)
					log.Fatalln(err)
				}
			}
			return nil
		}),
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

func gatherTestPageUrls(specialtyUrls *[]string, nodeMap *map[int]*cdp.Node) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var retErr error
		var tmpNodes []*cdp.Node
		for _, specialtyUrl := range *specialtyUrls {
			specialtyUrl = fmt.Sprintf("%s&rows=4000", specialtyUrl)
			log.Printf("Visiting %s", specialtyUrl)
			retErr = errors.Join(retErr, chromedp.Run(
				ctx,
				chromedp.Navigate(specialtyUrl),
				chromedp.Sleep(10*time.Second),
				chromedp.Nodes("md-card", &tmpNodes),
			))

			for _, tmpNode := range tmpNodes {
				id := tmpNode.AttributeValue("id")
				idNum, err := strconv.Atoi(strings.TrimPrefix(id, "MASTER"))
				if err != nil {
					continue
				}
				(*nodeMap)[idNum] = tmpNode
			}
			log.Printf("number of md-cards: %d", len(*nodeMap))
			break
		}
		return retErr
	}
}

func gatherTestData(id int, testDataMap *map[int]TestData) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		var retErr error
		testData := TestData{}
		errors.Join(addName(ctx, &testData), retErr)
		(*testDataMap)[id] = testData
		return retErr
	}
}

func addName(ctx context.Context, testData *TestData) error {
	var nodes []*cdp.Node
	chromedp.Run(ctx, chromedp.Nodes(".qd-header__title-mobile.ng-binding", &nodes))
	if len(nodes) != 1 {
		return errors.New("not one node returned for name search")
	}
	for _, node := range nodes {
		if err := dom.RequestChildNodes(node.NodeID).WithDepth(-1).Do(ctx); err != nil {
			return errors.New("could nto request child nodes")
		}
	}

	node := nodes[0]
	for _, child := range node.Children {
		if child.NodeName == "#text" {
			testData.name = child.NodeValue
			break
		}
	}

	return nil
}
