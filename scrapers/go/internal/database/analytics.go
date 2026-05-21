package database

import (
	"context"
	"encoding/json"
	"fmt"
	"go-version/internal/models"
)

// GetLatestVercelSnapshot fetches the most recent Vercel analytics snapshot.
func (r *Repository) GetLatestVercelSnapshot(ctx context.Context) (*models.VercelStats, error) {
	query := `SELECT raw_json FROM vercel ORDER BY scraped_at DESC LIMIT 1`
	var rawJSON []byte
	err := r.db.QueryRow(ctx, query).Scan(&rawJSON)
	if err != nil {
		return nil, err
	}
	var stats models.VercelStats
	if err := json.Unmarshal(rawJSON, &stats); err != nil {
		return nil, err
	}
	return &stats, nil
}

// SaveVercelSnapshot inserts a new Vercel analytics snapshot row.
func (r *Repository) SaveVercelSnapshot(ctx context.Context, stats models.VercelStats) error {
	rawJSON, _ := json.Marshal(stats)
	query := `
		INSERT INTO vercel (visitors, page_views, bounce_rate, top_pages, referrers, countries, devices, os, raw_json)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.Exec(ctx, query,
		stats.Visitors, stats.PageViews, stats.BounceRate, stats.TopPages,
		stats.Referrers, stats.Countries, stats.Devices, stats.OS, rawJSON,
	)
	return err
}

// GetLatestCloudflareSnapshot fetches the most recent Cloudflare snapshot.
func (r *Repository) GetLatestCloudflareSnapshot(ctx context.Context) (*models.CloudflareStats, error) {
	query := `SELECT data_hash, last_sent_date, stats_json, total_requests FROM cloudflare ORDER BY scraped_at DESC LIMIT 1`
	var hash, lastSentDate string
	var statsJSON []byte
	var totalRequests int

	err := r.db.QueryRow(ctx, query).Scan(&hash, &lastSentDate, &statsJSON, &totalRequests)
	if err != nil {
		return nil, err
	}

	var stats map[string]models.CloudflareWorkerInvocation
	if err := json.Unmarshal(statsJSON, &stats); err != nil {
		return nil, err
	}

	return &models.CloudflareStats{
		Hash:          hash,
		LastSentDate:  lastSentDate,
		TotalRequests: totalRequests,
		Stats:         stats,
	}, nil
}

// SaveCloudflareSnapshot inserts a new Cloudflare snapshot.
func (r *Repository) SaveCloudflareSnapshot(ctx context.Context, stats models.CloudflareStats) error {
	statsJSON, _ := json.Marshal(stats.Stats)
	query := `
		INSERT INTO cloudflare (total_requests, stats_json, data_hash, last_sent_date)
		VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(ctx, query, stats.TotalRequests, statsJSON, stats.Hash, stats.LastSentDate)
	return err
}

// LogIncident inserts an incident into scraper_incidents.
func (r *Repository) LogIncident(ctx context.Context, platform, incidentType, errorMsg, screenshotURL string) error {
	query := `
		INSERT INTO scraper_incidents (platform, incident_type, error_msg, screenshot_url)
		VALUES ($1, $2, $3, $4)`
	_, err := r.db.Exec(ctx, query, platform, incidentType, errorMsg, screenshotURL)
	if err != nil {
		return fmt.Errorf("failed to log incident: %w", err)
	}
	return nil
}
