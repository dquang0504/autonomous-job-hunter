// Define an interface for all scrapers
// Ensure consistency

package scraper

import (
	"context"

	"github.com/playwright-community/playwright-go"
)

type Job struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Company     string `json:"company"`
	URL         string `json:"url"`
	Location    string `json:"location"`
	Salary      string `json:"salary"`
	Techstack   string `json:"techstack"`
	Description string `json:"description"`
	Source      string `json:"source"`
	PostedDate  string `json:"posted_date"`
	MatchScore  int    `json:"match_score"`
}

// Scraper defines the interface that all platform scrapers must implement
type Scraper interface {
	//Scrape jobs from the platform
	Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]Job, error)

	//Name is the platform name (TopCV, Facebook, ...)
	Name() string
}
