package facebook

import (
	"context"
	"fmt"
	"go-version/internal/browser"
	"go-version/internal/classifier"
	"go-version/internal/config"
	"go-version/internal/filter"
	"go-version/internal/scraper"
	"log"
	"net/url"
	"regexp"
	"strings"

	"github.com/playwright-community/playwright-go"
)

type FacebookScraper struct {
	cfg *config.Config
}

func NewFacebookScraper(cfg *config.Config) *FacebookScraper {
	return &FacebookScraper{cfg: cfg}
}

func (s *FacebookScraper) Name() string {
	return "Facebook"
}

func (s *FacebookScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	log.Println("📘 Searching Facebook Groups (Authenticated)...")

	var allJobs []scraper.Job
	groups := s.cfg.FacebookGroups
	if len(groups) == 0 {
		log.Println("⚠️ No Facebook groups configured")
		return allJobs, nil
	}

	// Filter for 2026: start_year:2026, end_year:2026
	recentPostsFilter := "eyJyZWNlbnRfcG9zdHM6MCI6IntcIm5hbWVcIjpcInJlY2VudF9wb3N0c1wiLFwiYXJnc1wiOlwiXCJ9IiwicnBfY3JlYXRpb25fdGltZTowIjoie1wibmFtZVwiOlwiY3JlYXRpb25fdGltZVwiLFwiYXJnc1wiOlwie1xcXCJzdGFydF95ZWFyXFxcIjpcXFwiMjAyNlxcXCIsXFxcInN0YXJ0X21vbnRoXFxcIjpcXFwiMjAyNi0xXFxcIixcXFwiZW5kX3llYXJcXFwiOlxcXCIyMDI2XFxcIixcXFwiZW5kX21vbnRoXFxcIjpcXFwiMjAyNi0xMlxcXCIsXFxcInN0YXJ0X2RheVxcXCI6XFxcIjIwMjYtMS0xXFxcIixcXFwiZW5kX2RheVxcXCI6XFxcIjIwMjYtMTItMzFcXFwifVwifSJ9"
	searchKeyword := "golang"

	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %v", err)
	}
	defer page.Close()

	// --- WARM UP PHASE ---
	log.Println("🏠 Navigating to Facebook Home for warm-up...")
	if _, err := page.Goto("https://www.facebook.com", playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateDomcontentloaded}); err != nil {
		log.Printf("⚠️ Warm-up failed (non-critical): %v", err)
	} else {
		log.Println("⏳ Warming up for ~5s with random behaviors...")
		for i := 0; i < 5; i++ {
			browser.MouseJiggle(page)
			browser.RandomDelay(1000, 2000)
		}
		log.Println("✅ Warm-up complete. Starting scraping...")
	}
	// --- END WARM UP ---

	authIssueDetected := false

	// Build seed model for Naive Bayes filtering
	seedModel, err := classifier.BuildModel("internal/classifier/seeds.json")
	if err != nil {
		log.Printf("⚠️ Warning: Could not load Naive Bayes model: %v", err)
	}

	for _, groupUrl := range groups {
		if authIssueDetected {
			break
		}

		cleanGroupUrl := strings.TrimRight(groupUrl, "/")
		cleanGroupUrl = strings.ReplaceAll(cleanGroupUrl, "mbasic.facebook.com", "www.facebook.com")
		cleanGroupUrl = strings.ReplaceAll(cleanGroupUrl, "m.facebook.com", "www.facebook.com")

		searchUrl := fmt.Sprintf("%s/search?q=%s&filters=%s", cleanGroupUrl, url.QueryEscape(searchKeyword), recentPostsFilter)
		log.Printf("  👥 Visiting Group Search: %s | keyword=\"%s\"", cleanGroupUrl, searchKeyword)

		page.SetExtraHTTPHeaders(map[string]string{"Referer": cleanGroupUrl})
		_, err := page.Goto(searchUrl, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(30000),
		})
		page.SetExtraHTTPHeaders(map[string]string{})

		if err != nil {
			log.Printf("  ❌ Failed to goto search page: %v", err)
			continue
		}

		browser.RandomDelay(3500, 6500)
		browser.MouseJiggle(page)

		// Check for Blocked/Login
		pageUrl := page.URL()
		emailLocators, _ := page.Locator("input[name=\"email\"]").Count()
		if strings.Contains(pageUrl, "checkpoint") || emailLocators > 0 {
			log.Println("  ⛔ Checkpoint/Login detected. Skipping.")
			authIssueDetected = true
			break
		}

		log.Println("    ⏳ Loading posts...")
		// Human scroll 5 times
		for i := 0; i < 5; i++ {
			page.Evaluate("window.scrollBy(0, Math.floor(Math.random() * 400) + 200)")
			browser.RandomDelay(500, 1200)
		}
		browser.RandomDelay(3000, 5000)

		postSelector := `div[role="feed"] > div, div[role="article"]`
		locator := page.Locator(postSelector)
		postsCount, _ := locator.Count()
		log.Printf("    📄 Found %d potential posts in feed.", postsCount)

		maxPostsToCheck := postsCount
		if maxPostsToCheck > 5 {
			maxPostsToCheck = 5
		}

		validPostsInGroup := 0
		maxNewJobsPerGroup := 5

		for i := 0; i < maxPostsToCheck; i++ {
			if validPostsInGroup >= maxNewJobsPerGroup {
				break
			}

			post := locator.Nth(i)
			isVisible, _ := post.IsVisible()
			if isVisible {
				post.ScrollIntoViewIfNeeded()
				page.WaitForTimeout(500)
			}
			browser.RandomDelay(1200, 2800)

			// --- TIMESTAMP EXTRACTION START ---
			jobTime := "Recent"
			svgs, err := post.Locator(`span > span > svg, span > a > span[id] > span`).All()
			if err == nil {
				var targetSvg playwright.Locator
				for _, svg := range svgs {
					box, err := svg.BoundingBox()
					if err == nil && box.Width <= 20 && box.Height <= 20 {
						targetSvg = svg
						break
					}
				}

				if targetSvg != nil {
					targetSvg.ScrollIntoViewIfNeeded()
					page.WaitForTimeout(500)

					box, err := targetSvg.BoundingBox()
					if err == nil && box != nil {
						centerY := box.Y + (box.Height / 2)
						clickX := box.X - 20
						tooltipY := centerY + 25

						currentStart := box.X - 150
						currentEnd := box.X + 29
						satisfied := false
						attempts := 0

						for attempts < 5 && !satisfied {
							attempts++

							// 1. Trigger Tooltip
							page.Mouse().Move(clickX, centerY, playwright.MouseMoveOptions{Steps: playwright.Int(5)})
							page.WaitForTimeout(1000)
							page.Mouse().Move(clickX, tooltipY, playwright.MouseMoveOptions{Steps: playwright.Int(5)})
							page.WaitForTimeout(100)

							// 2. Select Text (Drag)
							page.Mouse().Move(currentStart, tooltipY)
							page.Mouse().Down()
							page.Mouse().Move(currentEnd, tooltipY, playwright.MouseMoveOptions{Steps: playwright.Int(25)})
							page.WaitForTimeout(100)

							selectedTextRaw, _ := page.Evaluate("window.getSelection().toString()")
							selectedText := ""
							if selectedTextRaw != nil {
								selectedText = selectedTextRaw.(string)
							}
							page.Mouse().Up()

							if len(strings.TrimSpace(selectedText)) > 3 {
								cleanT := strings.TrimSpace(selectedText)
								cleanT = strings.ReplaceAll(cleanT, "\n", " ")
								cleanT = strings.ReplaceAll(cleanT, "\r", "")

								// Simplified check, in Go we can just use the selected text if it's not too long
								if len(cleanT) > 250 {
									jobTime = "Recent"
									satisfied = true
									break
								}

								// Minimal refinement port (exact pixel logic might be too brittle, but we replicate the structure)
								PPC := 3.0
								addLeftPx := 0.0
								addRightPx := 0.0

								if regexp.MustCompile(`(?i)^u,`).MatchString(cleanT) {
									addLeftPx = 7 * PPC
								} else if regexp.MustCompile(`(?i)^ư,`).MatchString(cleanT) {
									addLeftPx = 6 * PPC
								} else if regexp.MustCompile(`(?i)^(ay|ai),`).MatchString(cleanT) {
									addLeftPx = 6 * PPC
								} else if regexp.MustCompile(`(?i)^(am|ăm),`).MatchString(cleanT) {
									addLeftPx = 6 * PPC
								} else if regexp.MustCompile(`(?i)^(at|ật),`).MatchString(cleanT) {
									addLeftPx = 7 * PPC
								} else if regexp.MustCompile(`(?i)^ba,`).MatchString(cleanT) {
									addLeftPx = 6 * PPC
								} else if regexp.MustCompile(`(?i)^hứ`).MatchString(cleanT) {
									addLeftPx = 3 * PPC
								} else if regexp.MustCompile(`(?i)^ủ`).MatchString(cleanT) {
									addLeftPx = 4 * PPC
								} else if regexp.MustCompile(`(?i)^ứ`).MatchString(cleanT) {
									addLeftPx = 4 * PPC
								} else if regexp.MustCompile(`^,`).MatchString(cleanT) {
									addLeftPx = 9 * PPC
								} else if regexp.MustCompile(`^\d`).MatchString(cleanT) {
									addLeftPx = 13 * PPC
								} else if regexp.MustCompile(`^T(\s|$)`).MatchString(cleanT) {
									addLeftPx = 4 * PPC
								} else if regexp.MustCompile(`^C(\s|$)`).MatchString(cleanT) {
									addLeftPx = 4 * PPC
								}

								if regexp.MustCompile(`(?i)lúc\s*$`).MatchString(cleanT) {
									addRightPx = 7 * PPC
								} else if regexp.MustCompile(`(?i)\d{4}\s*$`).MatchString(cleanT) {
									addRightPx = 13 * PPC
								} else if regexp.MustCompile(`(?i):\s*$`).MatchString(cleanT) {
									addRightPx = 3 * PPC
								} else if regexp.MustCompile(`(?i):\d\s*$`).MatchString(cleanT) {
									addRightPx = 1.5 * PPC
								}

								if addLeftPx > 0 || addRightPx > 0 {
									log.Printf("      ⚠️ Truncated. Adding: L+%.1fpx, R+%.1fpx", addLeftPx, addRightPx)
									currentStart -= addLeftPx
									currentEnd += addRightPx
									page.WaitForTimeout(500)
									continue
								}

								jobTime = cleanT
								satisfied = true
								log.Printf("      🕒 Extracted Time: \"%s\"", jobTime)
							} else {
								break
							}
						}
					}
				}
			}
			// --- TIMESTAMP EXTRACTION END ---

			// Extract URL
			var postUrl string
			links, _ := post.Locator(`a[href*="/posts/"], a[href*="/permalink/"], a[href*="/groups/"]`).All()
			for _, link := range links {
				href, _ := link.GetAttribute("href")
				if strings.Contains(href, "/posts/") || strings.Contains(href, "/permalink/") {
					postUrl = href
					break
				}
			}

			if postUrl != "" {
				if strings.HasPrefix(postUrl, "/") {
					postUrl = "https://www.facebook.com" + postUrl
				}
				// Remove tracking params
				postUrl = regexp.MustCompile(`(\?|&)__cft__.*$`).ReplaceAllString(postUrl, "")
				postUrl = regexp.MustCompile(`(\?|&)ref=.*$`).ReplaceAllString(postUrl, "")
			} else {
				continue
			}

			log.Printf("    🔍 Inspecting Post %d/%d: %s", i+1, maxPostsToCheck, postUrl)

			// OPEN NEW TAB
			detailPage, err := browserCtx.NewPage()
			if err != nil {
				continue
			}
			log.Println("      🚀 Navigating to detail page...")

			_, err = detailPage.Goto(postUrl, playwright.PageGotoOptions{
				WaitUntil: playwright.WaitUntilStateDomcontentloaded,
				Timeout:   playwright.Float(30000),
			})
			detailPage.WaitForTimeout(1500)

			detailUrl := detailPage.URL()
			emailLocs, _ := detailPage.Locator("input[name=\"email\"]").Count()
			if strings.Contains(detailUrl, "checkpoint") || emailLocs > 0 {
				authIssueDetected = true
				detailPage.Close()
				break
			}

			// Wait for main content
			detailPage.WaitForSelector(`div[data-ad-rendering-role="story_message"]`, playwright.PageWaitForSelectorOptions{State: playwright.WaitForSelectorStateVisible, Timeout: playwright.Float(5000)})
			log.Println("      📄 Detail page content loaded.")

			browser.MouseJiggle(detailPage)
			browser.RandomDelay(1500, 3200)

			bodyText := ""
			storyMsg := detailPage.Locator(`div[data-ad-rendering-role="story_message"]`)
			if c, _ := storyMsg.Count(); c > 0 {
				texts, _ := storyMsg.AllInnerTexts()
				bodyText = strings.Join(texts, "\n")
				log.Println("      🎯 Extracted from story_message container.")
			} else {
				mainRole := detailPage.Locator(`div[role="main"]`)
				if c, _ := mainRole.Count(); c > 0 {
					bodyText, _ = mainRole.InnerText()
				} else {
					bodyText, _ = detailPage.Locator("body").InnerText()
				}
			}

			// Clean UI patterns
			uiPatterns := []string{
				`(?s)Tất cả cảm xúc:.*$`,
				`(?s)All reactions:.*$`,
				`(?s)Facebook Facebook Facebook.*$`,
				`(?s)Viết câu trả lời\.\.\..*$`,
				`(?s)Viết bình luận công khai.*$`,
				`(?s)Write a comment.*$`,
				`(?s)Thích\s+Bình luận\s+Chia sẻ.*$`,
				`(?s)Like\s+Comment\s+Share.*$`,
			}
			for _, pat := range uiPatterns {
				re := regexp.MustCompile(pat)
				bodyText = re.ReplaceAllString(bodyText, "")
			}
			bodyText = strings.TrimSpace(bodyText)

			// Remove truncate noise
			noiseRe := regexp.MustCompile(`(?i)(?:\.\.\.|…)\s*(?:Xem thêm|See more)`)
			noiseLocs := noiseRe.FindAllStringIndex(bodyText, -1)
			if len(noiseLocs) > 0 {
				lastMatch := noiseLocs[len(noiseLocs)-1]
				bodyText = strings.TrimSpace(bodyText[lastMatch[1]:])
				log.Println("      ✂️ Truncated noise.")
			}

			title, _ := detailPage.Title()
			title = strings.ReplaceAll(title, " | Facebook", "")

			shortDesc := bodyText
			if len(bodyText) > 1500 {
				shortDesc = "..." + bodyText[len(bodyText)-1500:]
			}

			job := scraper.Job{
				Title:       title,
				Company:     "Facebook Group",
				URL:         postUrl,
				Description: shortDesc,
				Location:    "Unknown",
				Source:      "Facebook",
				Techstack:   "Golang",
				PostedDate:  jobTime,
			}

			// Filtering
			if !filter.ShouldIncludeJob(job) {
				log.Println("      ❌ Filtered out: Doesn't match keyword/level requirements")
				detailPage.Close()
				continue
			}

			// Social Hiring Classifier (Naive Bayes)
			if seedModel != nil {
				res := classifier.ClassifyWithSeedModel(seedModel, bodyText)
				// If strictly NOT hiring
				if !res.IsHiring && res.Confidence > 0.6 {
					log.Println("      ❌ Filtered out: Local ML says not a job post")
					detailPage.Close()
					continue
				}
			}

			job.MatchScore = filter.CalculateMatchScore(job)
			log.Printf("      ✅ Valid Job Found! Score: %d", job.MatchScore)
			allJobs = append(allJobs, job)
			validPostsInGroup++

			detailPage.Close()
			browser.RandomDelay(1500, 3000)
		}

		browser.RandomDelay(4000, 8000)
	}

	// Dedup internally
	uniqueJobs := make([]scraper.Job, 0)
	seen := make(map[string]bool)
	for _, j := range allJobs {
		if !seen[j.URL] {
			seen[j.URL] = true
			uniqueJobs = append(uniqueJobs, j)
		}
	}

	return uniqueJobs, nil
}
