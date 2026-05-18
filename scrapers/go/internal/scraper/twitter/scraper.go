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

var jobKeywordRegex = regexp.MustCompile(`(?i)\b(hiring|job|opening|developer|engineer|position|remote|golang|go backend)\b`)

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

func (s *TwitterScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	var jobs []scraper.Job
	log.Println("🐦 Searching X (Twitter)...")

	//init page
	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("twitter: failed to create page: %w", err)
	}
	defer page.Close()

	//Build search query — Twitter uses up to 3 keywords from config (same as Node.js source)
	keywords := s.cfg.Keywords
	if len(keywords) > 3 {
		keywords = keywords[:3]
	}
	// Wrap each keyword in quotes so Twitter searches exact phrases
	// e.g. ["golang"] → ["\"golang\""] → joined: `"golang"`
	quotedKeywords := make([]string, len(keywords))
	for i, k := range keywords {
		quotedKeywords[i] = fmt.Sprintf(`"%s"`, k) // closing quote was missing
	}
	keywordPart := strings.Join(quotedKeywords, " OR ")
	searchQuery := fmt.Sprintf(`(%s) (job OR hiring) (fresher OR junior OR intern) -senior`, keywordPart)
	log.Printf("  🔍 Query: %.60s...", searchQuery)

	//navigate to latest tweets
	searchURL := fmt.Sprintf("https://x.com/search?q=%s&f=live", url.QueryEscape(searchQuery))
	if _, err := page.Goto(searchURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		return nil, fmt.Errorf("twitter: navigation failed: %w", err)
	}
	browser.RandomDelay(1000, 2000)

	//wait for tweets
	page.WaitForSelector(`[data-testid="tweet"]`, playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	})

	//check login wall
	loginCount, _ := page.Locator(`[data-testid="LoginForm"]`).Count()
	if loginCount > 0 {
		log.Println("  ⚠️ Twitter requires login — skipping. Ensure cookies are valid.")
		return nil, nil //graceful skip
	}

	//human scroll
	browser.HumanScroll(page)

	//collect tweets
	tweetEls, _ := page.Locator(`[data-testid="tweet"]`).All()
	log.Printf("  📦 Found %d tweets", len(tweetEls))
	limit := 30
	if len(tweetEls) < limit {
		limit = len(tweetEls)
	}

	for i := 0; i < limit; i++ {
		tweet := tweetEls[i]
		text, err := tweet.Locator(`[data-testid="tweetText"]`).TextContent(playwright.LocatorTextContentOptions{
			Timeout: playwright.Float(1000),
		})
		// Skip tweets with no text, or text too short to be a real job post
		if err != nil || len(strings.TrimSpace(text)) < 20 {
			continue
		}

		// Pre-filter: skip tweets with no job-related keywords before heavier processing
		if !jobKeywordRegex.MatchString(text) {
			continue
		}

		if !filter.IsSocialHiringPost(text) {
			continue
		}

		// Call FastText via Python (mandatory accuracy path)
		res, err := classifier.ClassifyWithFastText(ctx, text)
		if err == nil {
			if !res.IsHiring && res.Confidence > 0.6 {
				log.Printf("      ❌ Tweet filtered out by FastText ML classifier")
				continue
			}
		} else {
			log.Printf("      ⚠️ FastText classifier failed: %v", err)
		}

		//extract fields
		authorHref, _ := tweet.Locator(`[data-testid="User-Name"] a`).First().GetAttribute("href")
		tweetHref, _ := tweet.Locator(`a[href*="/status"]`).First().GetAttribute("href")
		dateTime, _ := tweet.Locator("time").First().GetAttribute("datetime")

		//build title - first 100 runes (rune-safe for Unicode/emoji in tweets)
		title := strings.TrimSpace(text)
		if len([]rune(title)) > 100 {
			title = string([]rune(title)[:100]) + "..."
		}
		// TrimPrefix removes leading "/" from Twitter href e.g. "/username" → "username"
		company := strings.TrimPrefix(authorHref, "/")
		if company == "" {
			company = "Twitter Post"
		}
		// tweetHref is a relative path e.g. "/username/status/123" — must prefix with domain
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

		//build job
		jobs = append(jobs, scraper.Job{
			Title:       title,
			Description: text,
			Company:     company,
			URL:         jobURL,
			Location:    "Remote/Global",
			Salary:      filter.ExtractSalary(text),
			Source:      "X (Twitter)",
			Techstack:   "Golang",
			PostedDate:  postedDate,
			MatchScore:  5, //default
		})
		// %.40s truncates title to 40 chars for readable log output
		log.Printf("    📝 %.40s...", title)
	}
	log.Printf("  📊 Collected %d tweets", len(jobs))

	// AI Batch Validation
	if s.aiClient != nil && len(jobs) > 0 {
		jobs, err = s.aiClient.ValidateSocialJobsBatch(ctx, jobs)
		if err != nil {
			log.Printf("      ⚠️ AI Validation Error: %v", err)
			// Return jobs anyway as fallback
		}
	}

	return jobs, nil
}
