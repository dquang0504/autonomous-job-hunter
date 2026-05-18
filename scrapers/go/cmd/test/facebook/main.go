package main

import (
	"context"
	"fmt"
	"go-version/internal/ai"
	"go-version/internal/browser"
	"go-version/internal/config"
	"go-version/internal/scraper/facebook"
	"go-version/internal/telegram"
	"log"
	"os"
	"path/filepath"
	"time"

)

func main() {
	fmt.Println("📘 Starting Quick Facebook Scraper Test...")

	// 1. Load config
	cfg := config.Load()

	// For a quick test, we can limit the groups to only the first one to make it fast
	if len(cfg.FacebookGroups) > 0 {
		fmt.Printf("ℹ️ Found %d configured Facebook groups. Limiting to the first group for a quick test.\n", len(cfg.FacebookGroups))
		cfg.FacebookGroups = []string{cfg.FacebookGroups[0]}
		fmt.Printf("👥 Testing group: %s\n", cfg.FacebookGroups[0])
	} else {
		log.Fatal("❌ No Facebook groups configured in configs/config.yaml")
	}

	// 2. Initialize Playwright
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pm, err := browser.NewPlaywright(ctx)
	if err != nil {
		log.Fatalf("❌ Failed to create Playwright: %v", err)
	}
	defer pm.Close()
	fmt.Println("✅ Playwright started")

	// 3. Load cookies
	cookieFile := filepath.Join(cfg.CookiesPath, "cookies-facebook.json")
	fmt.Printf("🍪 Loading cookies from: %s\n", cookieFile)
	cookies, err := browser.LoadCookies(cookieFile)
	if err != nil {
		log.Fatalf("❌ Failed to load Facebook cookies: %v", err)
	}
	fmt.Printf("✅ Loaded %d cookies\n", len(cookies))

	browserCtx, err := pm.NewContext(cookies)
	if err != nil {
		log.Fatalf("❌ Failed to create browser context: %v", err)
	}
	defer browserCtx.Close()
	fmt.Println("✅ Browser context created")

	// 4. Initialize AI Client if available
	var aiClient *ai.GrokClient
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey != "" {
		aiClient = ai.NewGrokClient(groqKey)
		fmt.Println("🤖 AI Validator enabled for test")
	}

	// 5. Initialize Facebook Scraper
	scraperInstance := facebook.NewFacebookScraper(cfg, aiClient)

	// 6. Run Scrape
	fmt.Println("🔍 Scraping Facebook...")
	jobs, err := scraperInstance.Scrape(ctx, browserCtx)
	if err != nil {
		log.Fatalf("❌ Facebook scraper failed: %v", err)
	}

	fmt.Printf("🎉 Scraped %d valid job(s) from Facebook!\n", len(jobs))

	// 7. Initialize Telegram Bot
	fmt.Println("✉️ Connecting to Telegram Bot...")
	bot, err := telegram.NewBot(cfg.TelegramToken, cfg.TelegramChatID)
	if err != nil {
		log.Fatalf("❌ Failed to connect to Telegram: %v", err)
	}
	fmt.Println("✅ Connected to Telegram Bot")

	// Send status report
	statusMsg := fmt.Sprintf("Facebook Scraper test run completed. Found %d valid job(s).", len(jobs))
	_ = bot.SendStatus(statusMsg)

	// 8. Wire results to Telegram
	for _, job := range jobs {
		fmt.Printf("📤 Sending job to Telegram: %s at %s\n", job.Title, job.Company)
		err := bot.SendJob(job, "")
		if err != nil {
			log.Printf("⚠️ Failed to send job to Telegram: %v", err)
		} else {
			fmt.Println("✅ Job sent successfully!")
		}
	}

	fmt.Println("✨ Facebook Scraper test completed successfully!")
}
