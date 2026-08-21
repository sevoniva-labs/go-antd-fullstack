package datapolicy

import (
	"context"
	"time"
)

func (r *memoryArtifactRegistry) MarkCleanupPending(_ context.Context, organizationID, artifactID string, now time.Time) (DownloadArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[artifactID]
	if !ok || item.OrganizationID != organizationID {
		return DownloadArtifact{}, ErrDownloadArtifactNotFound
	}
	if item.Status != DownloadArtifactRevoked && item.Status != DownloadArtifactExpired {
		return DownloadArtifact{}, ErrDownloadArtifactInvalid
	}
	item.Status = DownloadArtifactCleanup
	item.UpdatedAt = now.UTC()
	r.items[artifactID] = item
	return item, nil
}

func (r *memoryArtifactRegistry) CompleteCleanup(_ context.Context, organizationID, artifactID string, now time.Time) (DownloadArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[artifactID]
	if !ok || item.OrganizationID != organizationID {
		return DownloadArtifact{}, ErrDownloadArtifactNotFound
	}
	if item.Status != DownloadArtifactCleanup {
		return DownloadArtifact{}, ErrDownloadArtifactInvalid
	}
	if item.RevokedAt.IsZero() {
		item.Status = DownloadArtifactExpired
	} else {
		item.Status = DownloadArtifactRevoked
	}
	item.UpdatedAt = now.UTC()
	r.items[artifactID] = item
	return item, nil
}
