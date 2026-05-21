package database

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

// UploadScreenshot uploads a file to Supabase Storage and returns the public URL.
func (r *Repository) UploadScreenshot(ctx context.Context, filePath, filename, supabaseURL, supabaseKey string) (string, error) {
	if supabaseURL == "" || supabaseKey == "" {
		return "", fmt.Errorf("supabase URL or Service Key missing")
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	uploadURL := fmt.Sprintf("%s/storage/v1/object/screenshots/%s", supabaseURL, filename)
	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewReader(content))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "image/png")
	req.Header.Set("apikey", supabaseKey)
	req.Header.Set("Authorization", "Bearer "+supabaseKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	publicURL := fmt.Sprintf("%s/storage/v1/object/public/screenshots/%s", supabaseURL, filename)
	return publicURL, nil
}
