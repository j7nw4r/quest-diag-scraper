package main

import (
	"context"
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

	if err := chromedp.Run(ctx, scrape()); err != nil {
		log.Fatalln(err)
	}
}

func scrape() chromedp.Tasks {
	return chromedp.Tasks{}
}
