package main

import (
	"encoding/json"
	"fmt"
	"go-version/internal/scraper"
	"log"
	"os"
	"path/filepath"
	"time"
)

// saveJobs writes the list of scraped jobs to a local JSON log file for auditing.
// The file is named by today's date: logs/job-search-YYYY-MM-DD.json
func saveJobs(jobs []scraper.Job) {
	if len(jobs) == 0 {
		log.Println("ℹ️ No jobs to save.")
		return
	}

	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("⚠️ Failed to create logs directory: %v", err)
		return
	}

	filename := fmt.Sprintf("job-search-%s.json", time.Now().Format("2006-01-02"))
	filePath := filepath.Join(logDir, filename)

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		log.Printf("⚠️ Failed to marshal jobs to JSON: %v", err)
		return
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		log.Printf("⚠️ Failed to write logs file: %v", err)
		return
	}

	log.Printf("📁 Results saved to %s", filePath)
}

// extractExternalID returns a stable unique identifier for a job listing.
// We use the job URL as the external ID because:
//   - The URL is already unique per listing on each platform.
//   - Most scrapers don't expose the platform's internal numeric job ID directly.
//   - Combined with the `source` field in the DB UNIQUE(source, external_id) constraint,
//     it guarantees correct deduplication at the database level.
//
// Future improvement: parse the platform-specific numeric ID from the URL
// (e.g., LinkedIn job ID, ITViec job ID) for a cleaner external_id value.
func extractExternalID(jobURL string) string {
	return jobURL
}
