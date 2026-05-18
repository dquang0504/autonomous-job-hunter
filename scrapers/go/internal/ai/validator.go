package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-version/internal/scraper"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

type ValidationResult struct {
	ValidJobIndices []int `json:"valid_job_indices"`
}

// ValidateSocialJobsBatch takes a slice of jobs, sends their descriptions to Groq in a single batch,
// and returns only the jobs that the AI deems as genuine job posts.
func (c *GrokClient) ValidateSocialJobsBatch(ctx context.Context, jobs []scraper.Job) ([]scraper.Job, error) {
	if len(jobs) == 0 {
		return jobs, nil
	}

	// Prepare payload for AI
	type AIPost struct {
		ID   int    `json:"id"`
		Text string `json:"text"`
	}

	var batch []AIPost
	for i, j := range jobs {
		text := j.Description
		// truncate to save tokens
		if len(text) > 1000 {
			text = text[:1000]
		}
		batch = append(batch, AIPost{ID: i, Text: text})
	}

	batchJSON, _ := json.Marshal(batch)

	prompt := fmt.Sprintf(`
You are an expert technical recruiter analyzing social media posts.
You are given a JSON array of posts. Evaluate if EACH post is a genuine JOB HIRING ADVERTISEMENT.

A genuine job hiring advertisement MUST be:
1. A company, recruiter, or person explicitly looking to hire someone.
2. Contains an open position or role.

Strictly EXCLUDE:
- Tutorials, courses, coding tips
- Open source project promotions (e.g., "Check out my new repo", "I built a CLI tool")
- Discussions, architectural debates, personal opinions (e.g., "Rust vs Go")
- Candidates looking for jobs (e.g., "I am looking for work", "Hire me")
- News or generic tech updates.

Input:
%s

Output ONLY a JSON object containing a single key "valid_job_indices" whose value is an array of integers representing the IDs of the posts that ARE genuine job advertisements.
Example Output:
{"valid_job_indices": [0, 2, 3]}
If none are jobs, output {"valid_job_indices": []}.
`, string(batchJSON))

	reqBody := groqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []groqMessage{
			{Role: "system", Content: "You are a precise classifier. Always output strictly valid JSON conforming to the requested schema."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.1,
		JSONMode:    true,
	}

	// Because Groq supports response_format: {"type": "json_object"}
	reqBodyBytes, _ := json.Marshal(map[string]interface{}{
		"model":       reqBody.Model,
		"messages":    reqBody.Messages,
		"temperature": reqBody.Temperature,
		"response_format": map[string]string{
			"type": "json_object",
		},
	})

	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(reqBodyBytes))
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	log.Printf("    🧠 Sending batch of %d jobs to Groq AI for validation...", len(jobs))
	start := time.Now()
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("groq api error (%d): %s", resp.StatusCode, string(body))
	}

	var groqResp groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return nil, err
	}

	if len(groqResp.Choices) == 0 {
		return nil, fmt.Errorf("no response from groq")
	}

	content := groqResp.Choices[0].Message.Content
	
	// Sometimes AI wraps JSON in markdown blocks
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")

	var result ValidationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("failed to parse AI validation response: %v | Raw: %s", err, content)
	}

	log.Printf("    ✅ AI Validation took %v. AI approved %d/%d posts.", time.Since(start), len(result.ValidJobIndices), len(jobs))

	// Map indices back to jobs
	validSet := make(map[int]bool)
	for _, idx := range result.ValidJobIndices {
		validSet[idx] = true
	}

	var finalJobs []scraper.Job
	for i, j := range jobs {
		if validSet[i] {
			finalJobs = append(finalJobs, j)
		} else {
			log.Printf("      ❌ AI Rejected: %s", j.Title)
		}
	}

	return finalJobs, nil
}
