package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go-version/internal/models"
	"io"
	"net/http"
	"time"
)

type Client interface {
	TailorResume(ctx context.Context, masterResumeJSON string, jobDescription string) (*models.Resume, error)
}

type GrokClient struct {
	apiKey     string
	httpClient *http.Client
}

func NewGrokClient(apiKey string) *GrokClient {
	return &GrokClient{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqRequest struct {
	Model       string        `json:"model"`
	Messages    []groqMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	JSONMode    bool          `json:"json_mode,omitempty"`
}

type groqResponse struct {
	Choices []struct {
		Message groqMessage `json:"message"`
	} `json:"choices"`
}

func (c *GrokClient) TailorResume(ctx context.Context, masterResumeJSON string, jobDescription string) (*models.Resume, error) {
	prompt := fmt.Sprintf(`
You are an expert technical recruiter and resume writer. 
Your task is to tailor a master resume JSON to match a specific Job Description (JD).

Master Resume (JSON):
%s

Job Description:
%s

CRITICAL RULES:
1. Return ONLY a valid JSON object matching the Resume schema.
2. ABSOLUTELY FORBIDDEN to invent, hallucinate, or add any skills, tools, or experiences that are NOT present in the Master Resume.
3. If a skill is required in the JD but NOT listed in the Master Resume, you MUST NOT add it.
4. Your job is ONLY to SELECT, PRIORITIZE, and REORGANIZE existing information from the Master Resume to best fit the JD.
5. Highlight and prioritize existing skills, projects, and responsibilities that are directly relevant to the JD.
6. Rewrite the 'Summary' using ONLY facts present in the Master Resume, tailored to show why the candidate fits the JD.
7. Ensure the 'skills' section only contains items from the Master Resume, with JD-relevant ones moved to the top of their respective lists.
`, masterResumeJSON, jobDescription)

	reqBody := groqRequest{
		Model: "llama-3.3-70b-versatile",
		Messages: []groqMessage{
			{Role: "system", Content: "You are a professional resume tailoring assistant. Always output JSON."},
			{Role: "user", Content: prompt},
		},
		Temperature: 0.2,
	}

	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequestWithContext(ctx, "POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(jsonData))
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

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
	
	// Try to parse the JSON
	var tailored models.Resume
	if err := json.Unmarshal([]byte(content), &tailored); err != nil {
		// Fallback: try to find JSON block if AI added conversational text
		return nil, fmt.Errorf("failed to parse AI response as Resume JSON: %v", err)
	}

	return &tailored, nil
}
