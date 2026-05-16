package vietnamworks

import (
	"context"
	"fmt"
	"go-version/internal/browser"
	"go-version/internal/config"
	"go-version/internal/scraper"
	"log"
	"strings"

	"github.com/playwright-community/playwright-go"
)

type VietnamWorksScraper struct {
	cfg *config.Config
}

func NewVietnamWorksScraper(cfg *config.Config) *VietnamWorksScraper {
	return &VietnamWorksScraper{cfg: cfg}
}

func (s *VietnamWorksScraper) Name() string {
	return "VietnamWorks"
}

func (s *VietnamWorksScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	log.Println("🇻🇳 Searching VietnamWorks...")

	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %v", err)
	}
	defer page.Close()

	// URL provided by user
	searchUrl := "https://www.vietnamworks.com/viec-lam?q=golang&l=29.15&sortBy=date"
	log.Printf("  🔍 URL: %s", searchUrl)

	if _, err := page.Goto(searchUrl, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(60000),
	}); err != nil {
		return nil, fmt.Errorf("failed to goto search URL: %v", err)
	}

	// Stealth: Settle time
	browser.RandomDelay(3000, 5000)

	// Stealth: Human-like scroll
	if err := browser.HumanScroll(page); err != nil {
		log.Printf("⚠️ Warning: Stealth scroll failed: %v", err)
	}
	browser.RandomDelay(1000, 2000)

	// Extract jobs via Evaluate
	jobData, err := page.Evaluate(`() => {
		const results = [];
		const cards = document.querySelectorAll('div.search_list.view_job_item');
		
		cards.forEach(card => {
			try {
				const titleAnchor = card.querySelector('h2 a');
				const title = titleAnchor?.innerText?.replace(/^Mới\s+/, '').trim();
				const url = titleAnchor?.href;
				
				if (!title || !url) return;

				const companyAnchor = card.querySelector('div[class*="gUZzDT"] a, a[title][href*="/nha-tuyen-dung/"]');
				const company = companyAnchor?.innerText?.trim() || 'Unknown Company';

				const salarySpan = card.querySelector('span[class*="cfzaBi"]');
				const salary = salarySpan?.innerText?.trim() || 'Negotiable';

				const locationSpan = card.querySelector('span[class*="kVIiDJ"]');
				const location = locationSpan?.innerText?.trim() || 'Unknown';

				const dateText = card.querySelector('div[class*="cOFrSM"]')?.innerText?.trim() || 'Today';

				const techTags = Array.from(card.querySelectorAll('label[class*="jJOvRn"]'))
					.map(lbl => lbl.title || lbl.innerText)
					.filter(Boolean);

				results.push({
					title,
					url,
					company,
					location,
					salary,
					dateText,
					techTags
				});
			} catch (e) {}
		});
		return results;
	}`)

	if err != nil {
		return nil, fmt.Errorf("failed to evaluate job extraction: %v", err)
	}

	// Cast jobData to []interface{} and then to our Job struct
	rawJobs := jobData.([]interface{})
	log.Printf("  📦 Found %d job cards", len(rawJobs))

	var jobs []scraper.Job
	for _, raw := range rawJobs {
		item := raw.(map[string]interface{})

		title := item["title"].(string)
		company := item["company"].(string)
		url := item["url"].(string)
		location := item["location"].(string)
		salary := item["salary"].(string)
		dateText := item["dateText"].(string)

		var techTags []string
		for _, tag := range item["techTags"].([]interface{}) {
			techTags = append(techTags, tag.(string))
		}

		// Simple level detection
		lowerTitle := strings.ToLower(title)
		level := "Unknown"
		if strings.Contains(lowerTitle, "intern") || strings.Contains(lowerTitle, "fresher") || strings.Contains(lowerTitle, "trainee") {
			level = "Intern/Fresher"
		} else if strings.Contains(lowerTitle, "junior") || strings.Contains(lowerTitle, "entry") {
			level = "Junior"
		} else if strings.Contains(lowerTitle, "senior") || strings.Contains(lowerTitle, "lead") || strings.Contains(lowerTitle, "expert") {
			level = "Senior+"
		}

		jobs = append(jobs, scraper.Job{
			Title:       title,
			Company:     company,
			URL:         url,
			Location:    location,
			Salary:      salary,
			Source:      "VietnamWorks",
			Description: fmt.Sprintf("Salary: %s | Level: %s | Tags: %s | Posted: %s", salary, level, strings.Join(techTags, ", "), dateText),
			PostedDate:  dateText,
			Techstack:   strings.Join(techTags, ", "),
		})
	}

	return jobs, nil
}
