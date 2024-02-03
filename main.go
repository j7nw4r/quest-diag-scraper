package main

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"log"
	"os"
	"os/signal"
)

const root_url string = "https://www.questdiagnostics.com/healthcare-professionals/test-directory"

func main() {
	ctx, notifyCancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer notifyCancel()

	ctx, cdpCancel := chromedp.NewContext(ctx)
	defer cdpCancel()

	var buf []byte
	if err := chromedp.Run(ctx, scrape(&buf)); err != nil {
		log.Fatalln(err)
	}

	if err := os.WriteFile("fullScreenshot.png", buf, 0o644); err != nil {
		log.Fatal(err)
	}
}

func scrape(res *[]byte) chromedp.Tasks {
	var nodes []*cdp.Node
	return chromedp.Tasks{
		chromedp.Navigate(root_url),
		chromedp.FullScreenshot(res, 100),
		chromedp.Nodes("#a2zContainer > div.component-body-area > div.a2z-area", &nodes),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error

			for _, node := range nodes {
				fmt.Println(node.Attributes)
			}

			return err
		}),
	}
}
