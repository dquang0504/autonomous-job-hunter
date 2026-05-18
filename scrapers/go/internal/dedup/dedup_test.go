package dedup

import (
	"context"
	"testing"
)

func TestJSONDeduplicator(t *testing.T) {
	tempDir := t.TempDir()
	d := NewDeduplicator(nil, tempDir)

	ctx := context.Background()
	url := "https://example.com/job/1"

	// 1. Initial state should be unseen
	if d.IsSeen(ctx, url) {
		t.Errorf("expected URL %q to be unseen initially", url)
	}

	// 2. Add to seen cache
	err := d.Add(ctx, url)
	if err != nil {
		t.Fatalf("failed to add URL: %v", err)
	}

	// 3. Now it should be seen
	if !d.IsSeen(ctx, url) {
		t.Errorf("expected URL %q to be seen after Add", url)
	}

	// 4. Test loading from disk (recreate Deduplicator on the same temp dir)
	d2 := NewDeduplicator(nil, tempDir)
	if !d2.IsSeen(ctx, url) {
		t.Errorf("expected URL %q to persist on disk and be seen by new Deduplicator", url)
	}
}
