package vercel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go-version/internal/config"
	"go-version/internal/database"
	"go-version/internal/models"
	"go-version/internal/scraper"
	"go-version/internal/telegram"

	"github.com/playwright-community/playwright-go"
)

type VercelScraper struct {
	cfg  *config.Config
	repo *database.Repository
	bot  *telegram.Bot
}

func NewVercelScraper(cfg *config.Config, repo *database.Repository, bot *telegram.Bot) *VercelScraper {
	return &VercelScraper{
		cfg:  cfg,
		repo: repo,
		bot:  bot,
	}
}

func (s *VercelScraper) Name() string {
	return "vercel"
}

func (s *VercelScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	fmt.Println("📈 Checking Vercel Analytics...")

	page, err := browserCtx.NewPage()
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	targetUrl := "https://vercel.com/dquang0504s-projects/my-portfolio/analytics?period=24h"
	fmt.Printf("  🚀 Visiting Analytics: %s\n", targetUrl)

	if _, err := page.Goto(targetUrl, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(30000),
	}); err != nil {
		s.handleError(ctx, page, "vercel_timeout", err)
		return nil, nil
	}

	page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateDomcontentloaded,
	})
	time.Sleep(4 * time.Second)
	page.WaitForSelector("text=Visitors", playwright.PageWaitForSelectorOptions{
		Timeout: playwright.Float(10000),
	})

	// Check login state
	bodyText, _ := page.InnerText("body")
	if strings.Contains(bodyText, "Log in to Vercel") {
		fmt.Println("  ⚠️ Vercel login required. Cookies might be expired.")
		s.handleError(ctx, page, "vercel_auth", fmt.Errorf("login required"))
		return nil, nil
	}

	fmt.Println("  ✅ Access Vercel Dashboard/Analytics")

	// Parse metrics
	cleanBody := regexp.MustCompile(`\n+`).ReplaceAllString(bodyText, "\n")
	visitors := extractRegex(cleanBody, `Visitors\s*\n\s*([\d,.]+)`)
	pageViews := extractRegex(cleanBody, `Page Views\s*\n\s*([\d,.]+)`)
	bounceRate := extractRegex(cleanBody, `Bounce Rate\s*\n\s*([\d,.]+%?)`)

	if visitors == "N/A" {
		fmt.Println("  ⚠️ Visitors data is N/A (Load error?)")
		return nil, nil
	}

	topPages := extractTopItem(cleanBody, "Pages", "Referrers")
	referrers := extractTopItem(cleanBody, "Referrers", "Countries")
	countries := extractTopItem(cleanBody, "Countries", "Devices")
	devices := extractTopItem(cleanBody, "Devices", "Operating Systems")
	osData := extractTopItem(cleanBody, "Operating Systems", "End")

	currentStats := models.VercelStats{
		Visitors:   visitors,
		PageViews:  pageViews,
		BounceRate: bounceRate,
		TopPages:   topPages,
		Referrers:  referrers,
		Countries:  countries,
		Devices:    devices,
		OS:         osData,
	}

	statsJson, _ := json.MarshalIndent(currentStats, "", "  ")
	fmt.Printf("  📊 Current Vercel Stats:\n%s\n", string(statsJson))

	// DB Logic
	if s.repo != nil {
		cachedStats, err := s.repo.GetLatestVercelSnapshot(ctx)
		isDifferent := true
		if err == nil && cachedStats != nil {
			cachedJson, _ := json.Marshal(*cachedStats)
			currJson, _ := json.Marshal(currentStats)
			if string(cachedJson) == string(currJson) {
				isDifferent = false
			}
		}

		if isDifferent {
			fmt.Println("  🔔 Stats changed. Sending notification.")
			if s.bot != nil {
				msg := fmt.Sprintf(`📈 *Vercel Analytics Report* (24h)
👥 *Traffic*:
• Visitors: %s
• Views: %s
• Bounce: %s

📄 *Top Pages*:
%s

🌍 *Locations*:
%s

📱 *Tech*:
• Devices: %s
• OS: %s
• Referrers: %s`,
					visitors, pageViews, bounceRate,
					formatList(topPages), formatList(countries),
					devices, osData, referrers)
				s.bot.SendMarkdownToOwner(msg)
			}
			s.repo.SaveVercelSnapshot(ctx, currentStats)
		} else {
			fmt.Println("  Draws 💤 Stats identical to cache. Skipping notification.")
		}
	}

	return nil, nil
}

func (s *VercelScraper) handleError(ctx context.Context, page playwright.Page, contextName string, err error) {
	fmt.Printf("  ❌ Error: %v\n", err)
	filename := fmt.Sprintf("%s_%d.png", contextName, time.Now().Unix())
	filePath := filepath.Join(os.TempDir(), filename)
	
	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(filePath),
		FullPage: playwright.Bool(true),
	}); err == nil {
		if s.repo != nil {
			publicUrl, _ := s.repo.UploadScreenshot(ctx, filePath, filename, s.cfg.SupabaseURL, s.cfg.SupabaseKey)
			s.repo.LogIncident(ctx, "vercel", contextName, err.Error(), publicUrl)
		}
		if s.bot != nil {
			s.bot.SendPhotoToOwner(filePath, fmt.Sprintf("🔍 Debug Screenshot: %s", contextName))
		}
	}
}

func extractRegex(text, pattern string) string {
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		return matches[1]
	}
	return "N/A"
}

func extractTopItem(text, header, stopHeader string) string {
	startIndex := strings.Index(text, header)
	if startIndex == -1 {
		return "N/A"
	}
	startIndex += len(header)
	
	endIndex := -1
	if stopHeader != "End" {
		endIndex = strings.Index(text[startIndex:], stopHeader)
	}
	
	substring := text[startIndex:]
	if endIndex != -1 {
		substring = text[startIndex : startIndex+endIndex]
	}
	
	lines := strings.Split(substring, "\n")
	var validLines []string
	ignoreList := []string{"Routes", "Hostnames", "UTM Parameters", "Browsers", "Visitors", "VISITORS", "Page Views", "Bounce Rate"}
	
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		ignored := false
		for _, ig := range ignoreList {
			if l == ig {
				ignored = true
				break
			}
		}
		if !ignored {
			validLines = append(validLines, l)
		}
	}
	
	if len(validLines) > 0 {
		max := 3
		if len(validLines) < 3 {
			max = len(validLines)
		}
		return strings.Join(validLines[:max], ", ")
	}
	return "No data"
}

func formatList(items string) string {
	if items == "N/A" || items == "No data" {
		return "• " + items
	}
	parts := strings.Split(items, ", ")
	var result []string
	for _, p := range parts {
		result = append(result, "• "+p)
	}
	return strings.Join(result, "\n")
}
