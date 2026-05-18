package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"go-version/internal/ai"
	"go-version/internal/browser"
	"go-version/internal/config"
	"go-version/internal/database"
	"go-version/internal/dedup"
	"go-version/internal/filter"
	"go-version/internal/models"
	"go-version/internal/scraper"
	"go-version/internal/scraper/facebook"
	"go-version/internal/scraper/itviec"
	"go-version/internal/scraper/threads"
	"go-version/internal/scraper/topcv"
	"go-version/internal/scraper/twitter"
	"go-version/internal/scraper/vietnamworks"
	"os"
	"path/filepath"
	"time"

	"github.com/playwright-community/playwright-go"
)

const staleJobDays = 60 // Auto-cleanup jobs older than this many days

func main() {
	platformFlag := flag.String("platform", "all", "Platform to scrape (all, topcv, itviec, twitter, facebook)")
	flag.Parse()

	// Load config
	cfg := config.Load()

	// Context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// --- DB CONNECTION ---
	var repo *database.Repository
	if cfg.DatabaseURL != "" {
		var err error
		if repo, err = database.ConnectDB(ctx, cfg.DatabaseURL); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️ DB connection failed: %v. Running without DB persistence.\n", err)
		} else {
			defer repo.Close()
			fmt.Fprintln(os.Stderr, "✅ Connected to Supabase DB.")

			// Auto-cleanup stale jobs on every run
			deleted, err := repo.DeleteStaleJobs(ctx, staleJobDays)
			if err != nil {
				fmt.Fprintf(os.Stderr, "⚠️ Cleanup warning: %v\n", err)
			} else if deleted > 0 {
				fmt.Fprintf(os.Stderr, "🧹 Auto-cleanup: removed %d jobs older than %d days.\n", deleted, staleJobDays)
			}
		}
	} else {
		fmt.Fprintln(os.Stderr, "⚠️ DATABASE_URL not set. Running in JSON-only mode (no DB persistence).")
	}

	// Initialize Deduplicator (Unified database repository and JSON cache file bridge)
	dedupManager := dedup.NewDeduplicator(repo, cfg.CachePath)

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
		"facebook": filepath.Join(cfg.CookiesPath, "cookies-facebook.json"),
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

	// Init AI Client
	var aiClient *ai.GrokClient
	groqKey := os.Getenv("GROQ_API_KEY")
	if groqKey != "" {
		aiClient = ai.NewGrokClient(groqKey)
		fmt.Fprintln(os.Stderr, "🤖 AI Validator enabled (GROQ_API_KEY detected)")
	}

	// Initialize scrapers
	availableScrapers := map[string]scraper.Scraper{
		"topcv":        topcv.NewTopCVScraper(cfg),
		"itviec":       itviec.NewITViecScraper(cfg),
		"twitter":      twitter.NewTwitterScraper(cfg, aiClient),
		"vietnamworks": vietnamworks.NewVietnamWorksScraper(cfg),
		"facebook":     facebook.NewFacebookScraper(cfg, aiClient),
		"threads":      threads.NewThreadsScraper(cfg, aiClient),
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

		var validJobs []scraper.Job

		for i := range jobs {
			j := &jobs[i]

			// Filter out irrelevant jobs based on level and stack
			if !filter.ShouldIncludeJob(*j) {
				fmt.Fprintf(os.Stderr, "🗑️ Filtered out (not relevant): %s\n", j.Title)
				continue
			}

			if j.MatchScore == 0 {
				j.MatchScore = filter.CalculateMatchScore(*j)
			}

			// Unified Deduplication Check (DB or JSON file-cache depending on setup)
			if dedupManager.IsSeen(ctx, j.URL) {
				fmt.Fprintf(os.Stderr, "⏩ Already processed, skip: %s\n", j.URL)
				continue
			}

			if repo != nil {
				// DB Persistence Mode
				dbJob := &models.Job{
					Source:         j.Source,
					ExternalID:     j.URL,
					Title:          j.Title,
					Company:        j.Company,
					URL:            j.URL,
					Location:       j.Location,
					Salary:         j.Salary,
					MatchScore:     j.MatchScore,
					PostedAt:       j.PostedDate,
					DescriptionRaw: j.Description,
				}
				saved, err := repo.SaveJob(ctx, dbJob)
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠️ Failed to save job to DB: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "💾 Saved to DB: %s (id=%s)\n", saved.Title, saved.ID)
					j.ID = saved.ID
				}
			} else {
				// JSON Cache Fallback Mode (Deduplicate using local seen-jobs.json)
				err := dedupManager.Add(ctx, j.URL)
				if err != nil {
					fmt.Fprintf(os.Stderr, "⚠️ Failed to save to local seen-jobs cache: %v\n", err)
				} else {
					fmt.Fprintf(os.Stderr, "💾 Saved to local seen-jobs cache: %s\n", j.URL)
				}
			}

			validJobs = append(validJobs, *j)
		}

		allJobs = append(allJobs, validJobs...)
	}

	// Output results as JSON to stdout for the Agent to consume
	output, _ := json.MarshalIndent(allJobs, "", "  ")
	fmt.Println(string(output))
}
