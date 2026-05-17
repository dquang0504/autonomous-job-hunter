package threads

import (
	"context"
	"encoding/json"
	"fmt"
	"go-version/internal/ai"
	"go-version/internal/browser"
	"go-version/internal/config"
	"go-version/internal/filter"
	"go-version/internal/scraper"
	"log"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/playwright-community/playwright-go"
)

var jobKeywordRegex = regexp.MustCompile(`(?i)\b(golang|go\s?lang|go\s?dev|go\s?engineer|backend\s?go)\b`)

type ThreadsScraper struct {
	cfg      *config.Config
	aiClient *ai.GrokClient
}

func NewThreadsScraper(cfg *config.Config, aiClient *ai.GrokClient) *ThreadsScraper {
	return &ThreadsScraper{
		cfg:      cfg,
		aiClient: aiClient,
	}
}

func (s *ThreadsScraper) Name() string {
	return "Threads"
}

// recursive json extraction
func extractPostsFromJSON(data interface{}, posts *[]scraper.Job) {
	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			extractPostsFromJSON(item, posts)
		}
	case map[string]interface{}:
		// Check for post
		if postWrap, ok := v["post"].(map[string]interface{}); ok {
			processPostObj(postWrap, posts)
		} else {
			processPostObj(v, posts)
		}
		
		for key, val := range v {
			if key == "__typename" || key == "viewer" || key == "extensions" {
				continue
			}
			extractPostsFromJSON(val, posts)
		}
	}
}

func processPostObj(post map[string]interface{}, posts *[]scraper.Job) {
	idVal, hasId := post["id"]
	if !hasId {
		idVal, hasId = post["pk"]
	}
	if !hasId {
		return
	}

	captionStr := ""
	if capMap, ok := post["caption"].(map[string]interface{}); ok {
		if text, ok := capMap["text"].(string); ok {
			captionStr = text
		}
	} else if text, ok := post["text"].(string); ok {
		captionStr = text
	}

	userStr := ""
	if userMap, ok := post["user"].(map[string]interface{}); ok {
		if u, ok := userMap["username"].(string); ok {
			userStr = u
		}
	}

	if userStr == "" {
		return
	}
	if captionStr == "" && post["image_versions2"] == nil {
		return
	}

	var timestamp float64
	if t, ok := post["taken_at"].(float64); ok {
		timestamp = t
	} else if t, ok := post["timestamp"].(float64); ok {
		timestamp = t
	}

	codeStr := fmt.Sprintf("%v", idVal)
	if code, ok := post["code"].(string); ok {
		codeStr = code
	}

	*posts = append(*posts, scraper.Job{
		ID:          fmt.Sprintf("%v", idVal),
		Title:       "Golang Opportunity",
		Company:     "@" + userStr,
		URL:         fmt.Sprintf("https://www.threads.com/@%s/post/%s", userStr, codeStr),
		Description: captionStr,
		Source:      "Threads",
		PostedDate:  fmt.Sprintf("%f", timestamp),
	})
}

func (s *ThreadsScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %v", err)
	}
	defer page.Close()

	log.Println("  🔐 Checking Threads authentication status...")
	_, err = page.Goto("https://www.threads.com/", playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	})
	if err != nil {
		return nil, err
	}
	
	time.Sleep(3 * time.Second)
	if strings.Contains(page.URL(), "/login") {
		return nil, fmt.Errorf("login wall detected")
	}

	var allJobs []scraper.Job
	keywords := s.cfg.Keywords
	if len(keywords) == 0 {
		return nil, fmt.Errorf("no keywords configured")
	}

	twoMonthsMs := int64(60 * 24 * 60 * 60)
	cutoffDate := time.Now().Unix() - twoMonthsMs

	var capturedResponses []interface{}
	var mu sync.Mutex

	page.OnResponse(func(response playwright.Response) {
		urlStr := response.URL()
		if strings.Contains(urlStr, "/api/graphql") || strings.Contains(urlStr, "searchResults") || strings.Contains(urlStr, "search_serp") {
			if strings.Contains(response.Headers()["content-type"], "application/json") {
				body, err := response.Body()
				if err == nil {
					var data interface{}
					if err := json.Unmarshal(body, &data); err == nil {
						mu.Lock()
						capturedResponses = append(capturedResponses, data)
						mu.Unlock()
					}
				}
			}
		}
	})

	for _, keyword := range keywords {
		log.Printf("  🔍 Searching Threads for: %s", keyword)
		searchUrl := fmt.Sprintf("https://www.threads.com/search?q=%s&serp_type=default&filter=recent", url.QueryEscape(keyword))
		
		_, err := page.Goto(searchUrl, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		})
		if err != nil {
			continue
		}
		
		time.Sleep(4 * time.Second)

		previousHeight := 0
		noChangeCount := 0
		seenPostIds := make(map[string]bool)

		for i := 0; i < 5; i++ {
			var keywordJobs []scraper.Job

			// Extract from captured responses
			mu.Lock()
			currentData := capturedResponses
			capturedResponses = nil // reset
			mu.Unlock()

			for _, d := range currentData {
				extractPostsFromJSON(d, &keywordJobs)
			}

			// Extract from scripts
			scripts, _ := page.QuerySelectorAll(`script[type="application/json"]`)
			for _, script := range scripts {
				content, _ := script.TextContent()
				var data interface{}
				if err := json.Unmarshal([]byte(content), &data); err == nil {
					extractPostsFromJSON(data, &keywordJobs)
				}
			}

			// Extract from DOM (Fallback)
			containers, _ := page.QuerySelectorAll(`div[data-pressable-container="true"]`)
			for _, container := range containers {
				text, _ := container.InnerText()
				if len(text) < 5 {
					continue
				}

				linkEl, _ := container.QuerySelector(`a[href*="/post/"]`)
				if linkEl == nil {
					continue
				}
				postUrl, _ := linkEl.GetAttribute("href")
				
				userEl, _ := container.QuerySelector(`a[href^="/@"]:not([href*="/post/"])`)
				username := "unknown"
				if userEl != nil {
					href, _ := userEl.GetAttribute("href")
					username = strings.ReplaceAll(strings.ReplaceAll(href, "/@", ""), "/", "")
				}

				keywordJobs = append(keywordJobs, scraper.Job{
					ID:          postUrl,
					Title:       "Golang Opportunity",
					Company:     "@" + username,
					URL:         "https://www.threads.com" + postUrl,
					Description: text,
					Source:      "Threads",
				})
			}

			newPosts := 0
			for _, job := range keywordJobs {
				if seenPostIds[job.ID] {
					continue
				}
				seenPostIds[job.ID] = true

				text := job.Description

				if !jobKeywordRegex.MatchString(text) {
					continue
				}

				if !filter.IsSocialHiringPost(text) {
					continue
				}

				var ts float64
				fmt.Sscanf(job.PostedDate, "%f", &ts)
				if ts > 0 && int64(ts) < cutoffDate {
					continue
				}

				job.Salary = filter.ExtractSalary(text)
				job.MatchScore = filter.CalculateMatchScore(job)
				
				// Optional: get first line of description as title if not set
				lines := strings.Split(text, "\n")
				if len(lines) > 0 {
					job.Title = lines[0]
					if len(job.Title) > 100 {
						job.Title = job.Title[:100]
					}
				}

				allJobs = append(allJobs, job)
				newPosts++
			}

			if newPosts == 0 {
				noChangeCount++
			} else {
				noChangeCount = 0
			}

			if noChangeCount >= 5 {
				break
			}

			browser.HumanScroll(page)
			time.Sleep(3 * time.Second)

			height, _ := page.Evaluate("document.body.scrollHeight")
			h := height.(int)
			if h == previousHeight {
				page.Evaluate("window.scrollTo(0, document.body.scrollHeight)")
				time.Sleep(2 * time.Second)
			}
			previousHeight = h
		}
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

	log.Printf("  📊 Collected %d Threads posts", len(uniqueJobs))

	// AI Batch Validation
	if s.aiClient != nil && len(uniqueJobs) > 0 {
		uniqueJobs, err = s.aiClient.ValidateSocialJobsBatch(ctx, uniqueJobs)
		if err != nil {
			log.Printf("      ⚠️ AI Validation Error: %v", err)
		}
	}

	return uniqueJobs, nil
}
