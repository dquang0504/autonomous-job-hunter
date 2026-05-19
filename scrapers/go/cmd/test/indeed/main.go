package main

import (
	"context"
	"fmt"
	"log"

	"go-version/internal/browser"
	"go-version/internal/config"
	"go-version/internal/scraper/indeed"
)

func main() {
	fmt.Println("🚀 Testing Indeed Scraper Phase 1 (Cloudflare & Cookie Check)")

	cfg := &config.Config{}

	// Load Playwright
	pm, err := browser.NewPlaywright(context.Background())
	if err != nil {
		log.Fatalf("❌ Playwright failed: %v", err)
	}
	defer pm.Close()

	// Load normalized cookies
	cookiePath := "../../.cookies/cookies-indeed.json"
	cookies, err := browser.LoadCookies(cookiePath)
	if err != nil {
		log.Printf("⚠️ Warning: Could not load cookies from %s: %v. Proceeding without cookies...", cookiePath, err)
	} else {
		fmt.Printf("🍪 Successfully loaded %d cookies for Indeed!\n", len(cookies))
	}

	// Create Stealth Context
	browserCtx, err := pm.NewContext(cookies)
	if err != nil {
		log.Fatalf("❌ Failed to create browser context: %v", err)
	}
	defer browserCtx.Close()

	scraperInstance := indeed.NewIndeedScraper(cfg)
	
	// Execute Phase 1
	_, err = scraperInstance.Scrape(context.Background(), browserCtx)
	if err != nil {
		log.Fatalf("❌ Scraper failed: %v", err)
	}

	fmt.Println("🎉 Phase 1 Test Completed!")
}
