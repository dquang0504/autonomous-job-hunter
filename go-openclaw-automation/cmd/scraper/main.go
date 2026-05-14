package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go-openclaw-automation/internal/browser"
	"go-openclaw-automation/internal/config"
	"go-openclaw-automation/internal/scraper"
	"go-openclaw-automation/internal/scraper/itviec"
	"go-openclaw-automation/internal/scraper/topcv"
	"go-openclaw-automation/internal/scraper/twitter"
	"go-openclaw-automation/internal/scraper/vietnamworks"
	"os"
	"path/filepath"
	"time"

	"github.com/playwright-community/playwright-go"
)

func main() {
	platformFlag := flag.String("platform", "all", "Platform to scrape (all, topcv, itviec, twitter)")
	flag.Parse()

	// Load config
	cfg := config.Load()

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Init playwright
	pwManager, err := browser.NewPlaywright(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to init Playwright: %v\n", err)
		os.Exit(1)
	}
	defer pwManager.Close()

	// Load cookies
	cookieFiles := map[string]string{
		"topcv":    filepath.Join(cfg.CookiesPath, "cookies-topcv.json"),
		"itviec":   filepath.Join(cfg.CookiesPath, "cookies-itviec.json"),
		"linkedin": filepath.Join(cfg.CookiesPath, "cookies-linkedin.json"),
		"twitter":  filepath.Join(cfg.CookiesPath, "cookies-twitter.json"),
	}
	var allCookies []playwright.OptionalCookie
	for name, cookieFile := range cookieFiles {
		cookies, err := browser.LoadCookies(cookieFile)
		if err == nil {
			allCookies = append(allCookies, cookies...)
		} else {
			fmt.Fprintf(os.Stderr, "⚠️ Warning: Could not load %s cookies\n", name)
		}
	}

	browserCtx, err := pwManager.NewContext(allCookies)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create browser context: %v\n", err)
		os.Exit(1)
	}

	// Initialize scrapers
	availableScrapers := map[string]scraper.Scraper{
		"topcv":        topcv.NewTopCVScraper(cfg),
		"itviec":       itviec.NewITViecScraper(cfg),
		"twitter":      twitter.NewTwitterScraper(cfg),
		"vietnamworks": vietnamworks.NewVietnamWorksScraper(cfg),
	}

	var activeScrapers []scraper.Scraper
	if *platformFlag == "all" {
		for _, s := range availableScrapers {
			activeScrapers = append(activeScrapers, s)
		}
	} else if s, ok := availableScrapers[*platformFlag]; ok {
		activeScrapers = append(activeScrapers, s)
	} else {
		fmt.Fprintf(os.Stderr, "❌ Unknown platform: %s\n", *platformFlag)
		os.Exit(1)
	}

	var allJobs []scraper.Job
	for _, s := range activeScrapers {
		fmt.Fprintf(os.Stderr, "▶️ Running scraper: %s\n", s.Name())
		jobs, err := s.Scrape(ctx, browserCtx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Scraper %s error: %v\n", s.Name(), err)
			continue
		}
		allJobs = append(allJobs, jobs...)
	}

	// Output results as JSON to stdout for the Agent to consume
	output, _ := json.MarshalIndent(allJobs, "", "  ")
	fmt.Println(string(output))
}
