package database

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type StorageClient struct {
	url    string
	key    string
	bucket string
	client *http.Client
}

func NewStorageClient(url, key, bucket string) *StorageClient {
	return &StorageClient{
		url:    url,
		key:    key,
		bucket: bucket,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// UploadPDF uploads a PDF byte array to Supabase Storage
func (s *StorageClient) UploadPDF(ctx context.Context, fileName string, content []byte) (string, error) {
	uploadURL := fmt.Sprintf("%s/storage/v1/object/%s/%s", s.url, s.bucket, fileName)

	req, err := http.NewRequestWithContext(ctx, "POST", uploadURL, bytes.NewBuffer(content))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/pdf")
	req.Header.Set("x-upsert", "true")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("storage upload failed (%d): %s", resp.StatusCode, string(body))
	}

	// The public URL (if bucket is public)
	publicURL := fmt.Sprintf("%s/storage/v1/object/public/%s/%s", s.url, s.bucket, fileName)
	return publicURL, nil
}

type storageObject struct {
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// DeleteStaleFiles removes files older than a certain duration from the bucket
func (s *StorageClient) DeleteStaleFiles(ctx context.Context, olderThan time.Duration) (int, error) {
	// 1. List files in bucket
	listURL := fmt.Sprintf("%s/storage/v1/object/list/%s", s.url, s.bucket)
	
	listReqBody := map[string]interface{}{
		"limit": 100,
		"offset": 0,
		"sortBy": map[string]string{"column": "name", "order": "asc"},
	}
	jsonBody, _ := json.Marshal(listReqBody)

	req, _ := http.NewRequestWithContext(ctx, "POST", listURL, bytes.NewBuffer(jsonBody))
	req.Header.Set("Authorization", "Bearer "+s.key)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("list files failed (%d)", resp.StatusCode)
	}

	var objects []storageObject
	if err := json.NewDecoder(resp.Body).Decode(&objects); err != nil {
		return 0, err
	}

	now := time.Now()
	var toDelete []string
	for _, obj := range objects {
		if now.Sub(obj.CreatedAt) > olderThan {
			toDelete = append(toDelete, obj.Name)
		}
	}

	if len(toDelete) == 0 {
		return 0, nil
	}

	// 2. Delete selected files
	deleteURL := fmt.Sprintf("%s/storage/v1/object/%s", s.url, s.bucket)
	deleteReqBody := map[string]interface{}{
		"prefixes": toDelete,
	}
	jsonDelBody, _ := json.Marshal(deleteReqBody)

	delReq, _ := http.NewRequestWithContext(ctx, "DELETE", deleteURL, bytes.NewBuffer(jsonDelBody))
	delReq.Header.Set("Authorization", "Bearer "+s.key)
	delReq.Header.Set("Content-Type", "application/json")

	delResp, err := s.client.Do(delReq)
	if err != nil {
		return 0, err
	}
	defer delResp.Body.Close()

	if delResp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("delete files failed (%d)", delResp.StatusCode)
	}

	return len(toDelete), nil
}
