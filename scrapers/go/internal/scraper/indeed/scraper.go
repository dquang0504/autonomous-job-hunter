package indeed

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"go-version/internal/browser"
	"go-version/internal/config"
	"go-version/internal/filter"
	"go-version/internal/scraper"
	"go-version/internal/text"

	"github.com/playwright-community/playwright-go"
)

var (
	highExperienceRegex = regexp.MustCompile(`(?i)\b([3-9]|\d{2,})\s*(\+|plus)?\s*(năm|nam|years?|yrs?|yoe)\b`)
	targetLevelRegex    = regexp.MustCompile(`(?i)\b(fresher|intern|junior|entry[\s-]?level|graduate|trainee)\b`)
	lowExperienceRegex  = regexp.MustCompile(`(?i)\b([01]|2)\s*(\+|plus)?\s*(năm|nam|years?|yrs?|yoe)\b`)
	excludeRegex        = regexp.MustCompile(`(?i)\b(senior|lead|manager|principal|staff|architect)\b`)
	antiTitleRegex      = regexp.MustCompile(`(?i)\b(frontend|front-end|ui/ux|qa|qc|tester|mobile|ios|android|flutter|react native|ba|business analyst|data analyst|data scientist|designer|devops|sysadmin|system admin|security|network|php|wordpress|magento|shopify|sales|marketing|hr)\b`)
	keywordRegex        = regexp.MustCompile(`(?i)\b(golang|go\s?lang|go\s?dev|go\s?engineer|backend\s?go)\b`)
	techStackRegex      = regexp.MustCompile(`(?i)\b(docker|kubernetes|aws|gcp|microservices|rest\s*api|grpc|backend|back-end|fullstack|full-stack)\b`)
	includeRegex        = regexp.MustCompile(`(?i)\b(fresher|intern|junior|entry[\s-]?level|graduate|trainee)\b`)
)

type IndeedScraper struct {
	cfg *config.Config
}

func NewIndeedScraper(cfg *config.Config) *IndeedScraper {
	return &IndeedScraper{
		cfg: cfg,
	}
}

func (s *IndeedScraper) Name() string {
	return "Indeed"
}

func shouldRejectForLevel(textVal string, title string) bool {
	normalizedText := text.Normalize(textVal)
	if normalizedText == "" {
		return false
	}

	// Rejection for level/experience (unless explicitly target level)
	hasDisqualifying := excludeRegex.MatchString(normalizedText) || highExperienceRegex.MatchString(normalizedText)
	hasTarget := targetLevelRegex.MatchString(normalizedText) || lowExperienceRegex.MatchString(normalizedText)
	if hasDisqualifying && !hasTarget {
		return true
	}

	if title != "" {
		normTitle := text.Normalize(title)
		if antiTitleRegex.MatchString(normTitle) && !keywordRegex.MatchString(normTitle) {
			return true
		}
	}

	return false
}

func calculateMatchScore(job scraper.Job) int {
	score := 0
	textVal := text.Normalize(job.Title + " " + job.Description + " " + job.Company)

	if keywordRegex.MatchString(textVal) {
		score += 3
	}
	if includeRegex.MatchString(textVal) {
		score += 3
	}

	// Location checks (Primary: HCM, Can Tho, Remote, Global)
	locLower := strings.ToLower(job.Location)
	primaryLocs := []string{"hcm", "ho chi minh", "saigon", "tphcm", "can tho", "cantho", "remote", "tu xa", "từ xa", "wfh"}
	for _, l := range primaryLocs {
		if strings.Contains(locLower, l) {
			score += 2
			break
		}
	}

	if techStackRegex.MatchString(textVal) {
		score += 1
	}

	if shouldRejectForLevel(textVal, job.Title) {
		score = 0
	}

	if score > 10 {
		return 10
	}
	if score < 0 {
		return 0
	}
	return score
}

func (s *IndeedScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	log.Printf("💼 Starting Indeed Go Scraper (DOM Search Inputs & Warm-Up)...")

	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Auto-close popups to avoid memory/tab leaks
	browserCtx.OnPage(func(p playwright.Page) {
		go func() {
			if p != nil {
				time.Sleep(1500 * time.Millisecond) // Give React handler time to run
				_ = p.Close()
			}
		}()
	})

	// --- WARM UP PHASE ---
	log.Println("🏠 Navigating to Indeed Home for warm-up...")
	if _, err := page.Goto("https://vn.indeed.com", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		log.Printf("⚠️ Warm-up failed (non-critical): %v", err)
	} else {
		log.Println("⏳ Warming up for ~5s with random behaviors...")
		for i := 0; i < 5; i++ {
			_ = browser.MouseJiggle(page)
			browser.RandomDelay(1000, 2000)
		}
		log.Println("✅ Warm-up complete. Starting search simulation...")
	}
	// --- END WARM UP ---

	// Resolve keywords and locations from config
	keywords := s.cfg.Keywords
	if len(keywords) == 0 {
		keywords = []string{"golang"}
	}

	locations := s.cfg.Locations
	if len(locations) == 0 {
		locations = []string{"Việt Nam", "Cần Thơ", "Remote"}
	}

	jobsMap := make(map[string]scraper.Job)
	seenKeys := make(map[string]bool)

	for _, keyword := range keywords {
		for _, locParam := range locations {
			fmt.Printf("\n🔄 Loop: Keyword=%q | Location=%q\n", keyword, locParam)

			// Ensure we are on an Indeed page that has the search inputs.
			// If not, navigate to home.
			hasInputs := false
			if whatEl, err := page.QuerySelector("#text-input-what"); err == nil && whatEl != nil {
				hasInputs = true
			}

			if !hasInputs {
				fmt.Println("  🌐 Navigating to Indeed Homepage...")
				_, err := page.Goto("https://vn.indeed.com", playwright.PageGotoOptions{
					WaitUntil: playwright.WaitUntilStateDomcontentloaded,
					Timeout:   playwright.Float(30000),
				})
				if err != nil {
					fmt.Printf("  ⚠️ Failed to navigate to homepage: %v. Skipping loop.\n", err)
					continue
				}
				browser.RandomDelay(2000, 4000)
			}

			// Clear and Type Keyword in "What" input
			fmt.Printf("  ⌨️ Entering search keyword: %q\n", keyword)
			whatInput := page.Locator("#text-input-what")
			if err := whatInput.WaitFor(playwright.LocatorWaitForOptions{Timeout: playwright.Float(5000)}); err != nil {
				fmt.Printf("  ⚠️ 'What' input not found: %v. Re-navigating to home...\n", err)
				_, _ = page.Goto("https://vn.indeed.com", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded})
				browser.RandomDelay(2000, 4000)
				whatInput = page.Locator("#text-input-what")
			}

			_ = whatInput.Click()
			browser.RandomDelay(200, 500)
			_ = page.Keyboard().Press("Control+A")
			browser.RandomDelay(100, 300)
			_ = page.Keyboard().Press("Backspace")
			browser.RandomDelay(200, 500)
			_ = whatInput.Type(keyword, playwright.LocatorTypeOptions{Delay: playwright.Float(120)})
			browser.RandomDelay(500, 1000)

			// Clear and Type Location in "Where" input
			fmt.Printf("  ⌨️ Entering search location: %q\n", locParam)
			whereInput := page.Locator("#text-input-where")
			_ = whereInput.Click()
			browser.RandomDelay(200, 500)
			_ = page.Keyboard().Press("Control+A")
			browser.RandomDelay(100, 300)
			_ = page.Keyboard().Press("Backspace")
			browser.RandomDelay(200, 500)
			_ = whereInput.Type(locParam, playwright.LocatorTypeOptions{Delay: playwright.Float(120)})
			browser.RandomDelay(800, 1500)

			// Press Enter to trigger search navigation
			fmt.Println("  🔍 Pressing Enter to search...")
			_ = page.Keyboard().Press("Enter")

			// Wait for search page results to load
			browser.RandomDelay(5000, 8000)
			_ = browser.MouseJiggle(page)

			// Cloudflare Check
			pageTitle, _ := page.Title()
			if strings.Contains(pageTitle, "Just a moment") || strings.Contains(pageTitle, "Challenge") {
				fmt.Println("  ⚠️ Cloudflare Turnstile detected! Waiting 8s for automatic session resolve...")
				page.WaitForTimeout(8000)
				_ = browser.MouseJiggle(page)
			}

			_ = browser.HumanScroll(page)

			// Selectors
			cardSelector := ".job_seen_beacon, .resultContent"
			jobCards, err := page.Locator(cardSelector).All()
			if err != nil || len(jobCards) == 0 {
				fmt.Printf("  ℹ️ No jobs found for %s in %s (URL: %s)\n", keyword, locParam, page.URL())
				continue
			}

			fmt.Printf("  📦 Found %d cards\n", len(jobCards))

			for idx := range jobCards {
				// Re-query locator to avoid stale elements after clicking
				freshCards, err := page.Locator(cardSelector).All()
				if err != nil || len(freshCards) <= idx {
					break
				}
				card := freshCards[idx]

				// Basic Info
				titleEl := card.Locator("h2.jobTitle span[title], a[id^=\"job_\"]").First()
				title, err := titleEl.TextContent()
				if err != nil || title == "" {
					continue
				}
				title = strings.TrimSpace(title)

				companyEl := card.Locator("[data-testid=\"company-name\"], .companyName").First()
				company := "Unknown"
				if companyEl != nil {
					if cText, err := companyEl.TextContent(); err == nil {
						company = strings.TrimSpace(cText)
					}
				}

				// Deduplication Key (Title + Company)
				uniqueKey := strings.ToLower(title + "|" + company)

				// Get URL
				linkEl := card.Locator("h2.jobTitle a, a[id^=\"job_\"]").First()
				url, err := linkEl.GetAttribute("href")
				if err != nil || url == "" {
					continue
				}
				if !strings.HasPrefix(url, "http") {
					url = "https://vn.indeed.com" + url
				}

				// Check if we already have this job
				if seenKeys[uniqueKey] {
					continue
				}

				locationEl := card.Locator("[data-testid=\"text-location\"], .companyLocation").First()
				locationRaw := "Vietnam"
				if locationEl != nil {
					if lText, err := locationEl.TextContent(); err == nil {
						locationRaw = strings.TrimSpace(lText)
					}
				}
				locationNorm := text.Normalize(locationRaw)

				// --- FAST FILTER ---
				if filter.IsHanoiOnly(locationNorm) {
					continue
				}

				// Check Exclude Title
				if shouldRejectForLevel(title, title) {
					fmt.Printf("    ❌ Skipped (Fast Title): %s\n", title)
					continue
				}

				// --- DEEP CHECK (Selective) ---
				fmt.Printf("    🔍 Verify: %s...\n", title)

				var description string
				var descLoadErr error

				// Click to load details (exact JS mirror)
				_ = card.ScrollIntoViewIfNeeded(playwright.LocatorScrollIntoViewIfNeededOptions{Timeout: playwright.Float(3000)})

				_ = browser.MouseJiggle(page)
				browser.RandomDelay(1000, 2000)

				// Try clicking linkEl; fallback to card click
				clickErr := linkEl.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(2000)})
				if clickErr != nil {
					clickErr = card.Click(playwright.LocatorClickOptions{
						Timeout: playwright.Float(2000),
						Force:   playwright.Bool(true),
					})
				}

				if clickErr == nil {
					descSelector := "#jobDescriptionText, .jobsearch-JobComponent-description"
					descEl, err := page.WaitForSelector(descSelector, playwright.PageWaitForSelectorOptions{
						Timeout: playwright.Float(4000),
					})
					if err == nil {
						description, descLoadErr = descEl.TextContent()
					} else {
						descLoadErr = err
					}
				} else {
					descLoadErr = clickErr
				}

				if descLoadErr != nil || len(description) < 50 {
					fmt.Printf("      ⚠️ Desc load failed/timeout for %s\n", title)
					continue
				}

				// Normalize full text including description
				jobTextNorm := text.Normalize(title + " " + company + " " + locationRaw + " " + description)

				// Re-check Keyword in Description
				if !keywordRegex.MatchString(jobTextNorm) {
					fmt.Printf("      ❌ Skipped (No Keyword in Desc)\n")
					continue
				}

				// Re-check Location in Description
				if filter.IsHanoiOnly(jobTextNorm) {
					fmt.Printf("      ❌ Skipped (Verify Loc Failed)\n")
					continue
				}

				job := scraper.Job{
					Title:       strings.TrimSpace(title),
					Company:     strings.TrimSpace(company),
					URL:         url,
					Salary:      "Negotiable",
					Location:    strings.TrimSpace(locationRaw),
					Source:      "Indeed",
					Techstack:   "Golang",
					Description: description,
				}

				job.MatchScore = calculateMatchScore(job)

				// Truncate description preview for console log
				preview := strings.ReplaceAll(job.Description, "\n", " ")
				preview = strings.ReplaceAll(preview, "\r", " ")
				if len(preview) > 120 {
					preview = preview[:120] + "..."
				}
				job.Description = preview

				// Add to map
				jobsMap[uniqueKey] = job
				seenKeys[uniqueKey] = true
				fmt.Printf("      ✅ MATCHED: %s\n", job.Title)

				// Delay between card clicks
				browser.RandomDelay(1500, 3000)
			}

			// Delay between search loops
			browser.RandomDelay(4000, 7000)
		}
	}

	// Convert map to slice
	var allJobs []scraper.Job
	for _, job := range jobsMap {
		allJobs = append(allJobs, job)
	}

	fmt.Printf("\n🎉 Phase 3 Complete! Total Deduplicated Match-Scraped Jobs: %d\n", len(allJobs))
	return allJobs, nil
}
