package itviec

import (
	"context"
	"go-version/internal/config"
	"testing"

	"github.com/playwright-community/playwright-go"
	"github.com/stretchr/testify/assert"
)

// setupPlaywright launches a real Chromium browser and returns a BrowserContext.
// The caller is responsible for defer browser.Close() and defer pw.Stop().
func setupPlaywright(t *testing.T) (*playwright.Playwright, playwright.Browser, playwright.BrowserContext) {
	t.Helper()

	pw, err := playwright.Run()
	if err != nil {
		t.Fatalf("could not launch playwright: %v", err)
	}

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		pw.Stop()
		t.Fatalf("could not launch browser: %v", err)
	}

	browserCtx, err := browser.NewContext()
	if err != nil {
		browser.Close()
		pw.Stop()
		t.Fatalf("could not create browser context: %v", err)
	}

	return pw, browser, browserCtx
}

// TestITViecScraper_Scrape_Cloudflare verifies that when responses look like a
// Cloudflare challenge page, Scrape returns an error (ITViec stops immediately
// on Cloudflare, unlike TopCV which skips gracefully).
func TestITViecScraper_Scrape_Cloudflare(t *testing.T) {
	pw, browser, browserCtx := setupPlaywright(t)
	defer pw.Stop()
	defer browser.Close()
	defer browserCtx.Close()

	// Intercept ALL requests and return a Cloudflare challenge page
	mockHTML := `<html><title>Attention Required! | Cloudflare</title><body><h1>Please verify you are a human</h1></body></html>`
	if err := browserCtx.Route("**/*", func(route playwright.Route) {
		route.Fulfill(playwright.RouteFulfillOptions{
			Status: playwright.Int(200),
			Body:   mockHTML,
		})
	}); err != nil {
		t.Fatalf("could not set up route interception: %v", err)
	}

	cfg := &config.Config{Keywords: []string{"golang"}}
	scraper := NewITViecScraper(cfg)

	_, err := scraper.Scrape(context.Background(), browserCtx)

	// ITViec returns an error when Cloudflare persists (hard stop, not graceful skip)
	assert.Error(t, err, "ITViec should return error when Cloudflare blocks")
}

// TestITViecScraper_Scrape_EmptyState verifies that when the page shows the
// empty-state element (no jobs found), Scrape returns 0 jobs and no error.
func TestITViecScraper_Scrape_EmptyState(t *testing.T) {
	pw, browser, browserCtx := setupPlaywright(t)
	defer pw.Stop()
	defer browser.Close()
	defer browserCtx.Close()

	// Return a page that looks like valid ITViec but with the empty state visible
	mockHTML := `<html><title>ITViec</title><body>
		<div data-jobs--filter-target="searchNoInfo">Không tìm thấy việc làm phù hợp</div>
	</body></html>`
	if err := browserCtx.Route("**/*", func(route playwright.Route) {
		route.Fulfill(playwright.RouteFulfillOptions{
			Status: playwright.Int(200),
			Body:   mockHTML,
		})
	}); err != nil {
		t.Fatalf("could not set up route interception: %v", err)
	}

	cfg := &config.Config{Keywords: []string{"golang"}}
	scraper := NewITViecScraper(cfg)

	jobs, err := scraper.Scrape(context.Background(), browserCtx)

	assert.NoError(t, err, "Should not return error for empty results")
	assert.Equal(t, 0, len(jobs), "Should return 0 jobs when empty state is shown")
}

// TestITViecScraper_Scrape_Real is an integration test that hits the real ITViec website.
//
// Run manually with: go test -v -run TestITViecScraper_Scrape_Real ./internal/scraper/itviec/
func TestITViecScraper_Scrape_Real(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode (-short flag). Run without -short to execute.")
	}

	pw, browser, browserCtx := setupPlaywright(t)
	defer pw.Stop()
	defer browser.Close()
	defer browserCtx.Close()

	cfg := &config.Config{
		Keywords: []string{"golang"},
	}
	scraper := NewITViecScraper(cfg)

	jobs, err := scraper.Scrape(context.Background(), browserCtx)

	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(jobs), 0, "Should return a non-negative number of jobs")
	t.Logf("Real scrape returned %d jobs", len(jobs))
}
