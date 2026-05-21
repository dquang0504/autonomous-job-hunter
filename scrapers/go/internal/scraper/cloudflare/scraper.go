package cloudflare

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go-version/internal/config"
	"go-version/internal/database"
	"go-version/internal/models"
	"go-version/internal/scraper"
	"go-version/internal/telegram"

	"github.com/playwright-community/playwright-go"
)

type CloudflareScraper struct {
	cfg  *config.Config
	repo *database.Repository
	bot  *telegram.Bot
}

func NewCloudflareScraper(cfg *config.Config, repo *database.Repository, bot *telegram.Bot) *CloudflareScraper {
	return &CloudflareScraper{
		cfg:  cfg,
		repo: repo,
		bot:  bot,
	}
}

func (s *CloudflareScraper) Name() string {
	return "cloudflare"
}

// Scrape uses HTTP directly (no browser needed)
func (s *CloudflareScraper) Scrape(ctx context.Context, browserCtx playwright.BrowserContext) ([]scraper.Job, error) {
	apiToken := os.Getenv("CLOUDFLARE_API_KEY")
	accountId := "05bdf9a77d8976b78faf594736063c5d"

	if apiToken == "" {
		fmt.Println("  ⚠️ CLOUDFLARE_API_KEY not found in env. Skipping...")
		return nil, nil
	}

	fmt.Println("🌩️ Checking Cloudflare Worker Analytics...")

	now := time.Now()
	past24h := now.Add(-24 * time.Hour)

	query := fmt.Sprintf(`
      query Viewer {
        viewer {
          accounts(filter: {accountTag: "%s"}) {
            workersInvocationsAdaptive(
              limit: 10,
              filter: {
                datetime_geq: "%s",
                datetime_leq: "%s"
              }
            ) {
              sum {
                requests
                errors
              }
              dimensions {
                scriptName
              }
            }
          }
        }
      }
    `, accountId, past24h.Format(time.RFC3339), now.Format(time.RFC3339))

	reqBody, _ := json.Marshal(map[string]string{"query": query})
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.cloudflare.com/client/v4/graphql", bytes.NewReader(reqBody))
	if err != nil {
		fmt.Printf("  ❌ Cloudflare Error: %v\n", err)
		return nil, nil
	}

	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("  ❌ Cloudflare API Error: %v\n", err)
		return nil, nil
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	type CFResponse struct {
		Errors []interface{} `json:"errors"`
		Data   struct {
			Viewer struct {
				Accounts []struct {
					WorkersInvocationsAdaptive []struct {
						Sum struct {
							Requests int `json:"requests"`
							Errors   int `json:"errors"`
						} `json:"sum"`
						Dimensions struct {
							ScriptName string `json:"scriptName"`
						} `json:"dimensions"`
					} `json:"workersInvocationsAdaptive"`
				} `json:"accounts"`
			} `json:"viewer"`
		} `json:"data"`
	}

	var parsed CFResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		fmt.Printf("  ❌ Cloudflare API Parse Error: %v\n", err)
		return nil, nil
	}

	if len(parsed.Errors) > 0 {
		fmt.Printf("  ❌ Cloudflare GraphQL Errors: %v\n", parsed.Errors)
		return nil, nil
	}

	if len(parsed.Data.Viewer.Accounts) == 0 {
		fmt.Println("  ℹ️ No Cloudflare Worker traffic in last 24h.")
		return nil, nil
	}

	data := parsed.Data.Viewer.Accounts[0].WorkersInvocationsAdaptive
	if len(data) == 0 {
		fmt.Println("  ℹ️ No Cloudflare Worker traffic in last 24h.")
		return nil, nil
	}

	currentStats := make(map[string]models.CloudflareWorkerInvocation)
	totalRequests := 0
	for _, item := range data {
		name := item.Dimensions.ScriptName
		reqs := item.Sum.Requests
		errs := item.Sum.Errors
		currentStats[name] = models.CloudflareWorkerInvocation{
			Requests: reqs,
			Errors:   errs,
		}
		totalRequests += reqs
	}

	statsJSON, _ := json.MarshalIndent(currentStats, "", "  ")
	fmt.Printf("  📊 Current Cloudflare Stats:\n%s\n", string(statsJSON))

	// Hash
	hashData, _ := json.Marshal(currentStats)
	hashBytes := sha256.Sum256(hashData)
	currentHash := hex.EncodeToString(hashBytes[:])

	// Get VN time (UTC+7)
	loc := time.FixedZone("VN", 7*3600)
	todayVN := now.In(loc).Format("2006-01-02")

	if s.repo != nil {
		cachedData, err := s.repo.GetLatestCloudflareSnapshot(ctx)
		if err != nil {
			cachedData = &models.CloudflareStats{}
		}

		isNewDay := cachedData.LastSentDate != todayVN
		isDataChanged := currentHash != cachedData.Hash
		hasTraffic := totalRequests > 0

		shouldNotify := hasTraffic && (isDataChanged || isNewDay)
		fmt.Printf("  📋 Cache check → today: %s, lastSentDate: %s, hashChanged: %v, newDay: %v, hasTraffic: %v\n",
			todayVN, cachedData.LastSentDate, isDataChanged, isNewDay, hasTraffic)

		if shouldNotify {
			reason := "📈 Traffic changed"
			if isNewDay && !isDataChanged {
				reason = "🗓️ New day — daily summary"
			}
			fmt.Printf("  🔔 Notifying: %s\n", reason)

			msg := fmt.Sprintf("🌩️ *Cloudflare Workers Report* (24h)\n_Reason: %s_\n", reason)
			for name, stats := range currentStats {
				msg += fmt.Sprintf("\n📦 *%s*:\n  • Requests: `%d`\n  • Errors: `%d`", name, stats.Requests, stats.Errors)
			}

			if s.bot != nil {
				s.bot.SendMarkdownToOwner(msg)
			}

			s.repo.SaveCloudflareSnapshot(ctx, models.CloudflareStats{
				Hash:          currentHash,
				LastSentDate:  todayVN,
				Timestamp:     now.Format(time.RFC3339),
				TotalRequests: totalRequests,
				Stats:         currentStats,
			})
			fmt.Printf("  ✅ Cache updated (hash + lastSentDate → %s).\n", todayVN)
		} else if !hasTraffic {
			fmt.Println("  💤 No traffic in last 24h. Skipping notification.")
		} else {
			fmt.Printf("  💤 Cloudflare data unchanged and already reported today (%s). Skipping.\n", todayVN)
		}
	}

	return nil, nil
}
