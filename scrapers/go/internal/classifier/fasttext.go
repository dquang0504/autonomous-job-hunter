package classifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type PythonPayload struct {
	Text string `json:"text"`
}

type PythonResponse struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	Margin     float64 `json:"margin"`
}

// ClassifyWithFastText executes the shared Python predictor script and routes text via stdin/stdout
func ClassifyWithFastText(ctx context.Context, text string) (ClassificationResult, error) {
	// Predict script is located at ../js/python/social_hiring_predict.py relative to scrapers/go
	scriptPath := "../js/python/social_hiring_predict.py"

	payload := PythonPayload{Text: text}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return ClassificationResult{}, err
	}

	// Invoke python3 out-of-process with a strict 10-second timeout
	cmdCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "python3", scriptPath)
	cmd.Stdin = bytes.NewReader(payloadBytes)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		return ClassificationResult{}, fmt.Errorf("python execution failed: %v, stderr: %s", err, stderr.String())
	}

	var resp PythonResponse
	if err := json.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return ClassificationResult{}, fmt.Errorf("failed to parse python output: %v, output: %s", err, stdout.String())
	}

	return ClassificationResult{
		Label:      resp.Label,
		IsHiring:   resp.Label == "hiring",
		Confidence: resp.Confidence,
		Margin:     resp.Margin,
		Source:     "fasttext",
	}, nil
}
