package datapolicy

import (
	"context"
	"time"
)

func (r *memoryArtifactRegistry) ListExpiredReady(_ context.Context, now time.Time, limit int) ([]DownloadArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if limit <= 0 {
		limit = 100
	}
	items := make([]DownloadArtifact, 0, limit)
	for _, item := range r.items {
		if item.Status != DownloadArtifactReady || item.ExpiresAt.After(now) {
			continue
		}
		items = append(items, item)
		if len(items) == limit {
			break
		}
	}
	return items, nil
}
