package twitter

import (
	"context"
	"fmt"
	"go-version/internal/ai"
	"go-version/internal/browser"
	"go-version/internal/classifier"
	"go-version/internal/config"
	"go-version/internal/filter"
	"go-version/internal/scraper"
	"log"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/playwright-community/playwright-go"
)

var jobKeywordRegex = regexp.MustCompile(`(?i)\b(hiring|job|opening|developer|engineer|position|remote|golang|go backend|go developer|backend role)\b`)

type TwitterScraper struct {
	cfg      *config.Config
	aiClient *ai.GrokClient
}

func NewTwitterScraper(cfg *config.Config, aiClient *ai.GrokClient) *TwitterScraper {
	return &TwitterScraper{
		cfg:      cfg,
		aiClient: aiClient,
	}
}

func (s *TwitterScraper) Name() string {
	return "X (Twitter)"
}

func extractLocation(text string) string {
	if filter.IsHanoiOnly(text) {
		return "Hanoi"
	}
	if filter.HasPreferredLocation(text) {
		// Just a heuristic matching JS roughly
		return "Ho Chi Minh"
	}

	// Tagged match
	taggedMatch := regexp.MustCompile(`[📍📌]\s*([^\n|•]{2,80})`).FindStringSubmatch(text)
	if len(taggedMatch) > 1 {
		return strings.TrimSpace(taggedMatch[1])
	}

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if regexp.MustCompile(`(?i)^(location|dia diem|địa điểm|based in|onsite in|hybrid in|work location)\b`).MatchString(line) {
			normalized := regexp.MustCompile(`(?i)^(location|dia diem|địa điểm|based in|onsite in|hybrid in|work location)\b`).ReplaceAllString(line, "")
			normalized = strings.Trim(normalized, " :,-")
			return strings.TrimSpace(normalized)
		}
	}

	return "Unknown"
}

func (s *TwitterScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	var jobs []scraper.Job
	log.Println("🐦 Searching X (Twitter)...")

	//init page
	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("twitter: failed to create page: %w", err)
	}
	defer page.Close()

	keywords := s.cfg.Keywords
	if len(s.cfg.SocialSearchKeywords) > 0 {
		keywords = s.cfg.SocialSearchKeywords
	}

	for _, keyword := range keywords {
		searchQuery := fmt.Sprintf(`"%s" (job OR hiring OR opening OR recruiter OR careers OR apply)`, keyword)
		log.Printf("  🔍 Query: %.80s...", searchQuery)

		searchURL := fmt.Sprintf("https://x.com/search?q=%s&f=live", url.QueryEscape(searchQuery))
		if _, err := page.Goto(searchURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(60000),
		}); err != nil {
			log.Printf("  ❌ Navigation failed: %v", err)
			continue
		}
		browser.RandomDelay(1000, 2000)

		page.WaitForSelector(`[data-testid="tweet"]`, playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(10000),
		})

		loginCount, _ := page.Locator(`[data-testid="LoginForm"]`).Count()
		if loginCount > 0 {
			log.Println("  ⚠️ Twitter requires login, skipping...")
			return jobs, nil
		}

		browser.HumanScroll(page)

		tweetEls, _ := page.Locator(`[data-testid="tweet"]`).All()
		log.Printf("  📦 Found %d tweets", len(tweetEls))
		limit := 7
		if len(tweetEls) < limit {
			limit = len(tweetEls)
		}

		for i := 0; i < limit; i++ {
			tweet := tweetEls[i]
			text, err := tweet.Locator(`[data-testid="tweetText"]`).TextContent(playwright.LocatorTextContentOptions{
				Timeout: playwright.Float(1000),
			})
			if err != nil || len(strings.TrimSpace(text)) < 20 {
				continue
			}

			if filter.IsHanoiOnly(text) || filter.HasExplicitNonPreferredLocation(text) {
				continue
			}

			if !jobKeywordRegex.MatchString(text) {
				continue
			}

			// Keep FastText model filter as per user request to "wire with .ftz"
			if filter.IsSocialHiringPost(text) {
				res, err := classifier.ClassifyWithFastText(ctx, text)
				if err == nil {
					if !res.IsHiring && res.Confidence > 0.6 {
						log.Printf("      ❌ Tweet filtered out by FastText ML classifier")
						continue
					}
				} else {
					log.Printf("      ⚠️ FastText classifier failed: %v", err)
				}
			}

			authorHref, _ := tweet.Locator(`[data-testid="User-Name"] a`).First().GetAttribute("href")
			tweetHref, _ := tweet.Locator(`a[href*="/status"]`).First().GetAttribute("href")
			dateTime, _ := tweet.Locator("time").First().GetAttribute("datetime")

			title := strings.TrimSpace(text)
			if len([]rune(title)) > 100 {
				title = string([]rune(title)[:100]) + "..."
			}

			company := strings.TrimPrefix(authorHref, "/")
			if company == "" {
				company = "Twitter Post"
			}

			jobURL := "https://x.com"
			if tweetHref != "" {
				jobURL = "https://x.com" + tweetHref
			}

			postedDate := "N/A"
			if dateTime != "" {
				t, err := time.Parse(time.RFC3339, dateTime)
				if err == nil {
					postedDate = t.Format("2006-01-02")
				}
			}

			job := scraper.Job{
				Title:       title,
				Description: text,
				Company:     company,
				URL:         jobURL,
				Location:    extractLocation(text),
				Salary:      filter.ExtractSalary(text),
				Source:      "X (Twitter)",
				Techstack:   "Go/Golang",
				PostedDate:  postedDate,
				MatchScore:  5,
			}

			job.MatchScore = filter.CalculateMatchScore(job)
			jobs = append(jobs, job)
			log.Printf("    📝 %.40s...", title)
		}
	}

	log.Printf("  📊 Collected %d tweets", len(jobs))

	// AI Batch Validation
	if s.aiClient != nil && len(jobs) > 0 {
		var validJobs []scraper.Job
		for _, job := range jobs {
			validJobs = append(validJobs, job)
		}
		// Actually validate using AI if available
		validatedJobs, err := s.aiClient.ValidateSocialJobsBatch(ctx, validJobs)
		if err == nil {
			jobs = validatedJobs
		} else {
			log.Printf("      ⚠️ AI Validation Error: %v", err)
		}
	}

	return jobs, nil
}
