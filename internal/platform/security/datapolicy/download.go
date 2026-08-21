package datapolicy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	DownloadArtifactReady      ArtifactStatus = "Ready"
	DownloadArtifactDownloaded ArtifactStatus = "Downloaded"
	DownloadArtifactRevoked    ArtifactStatus = "Revoked"
	DownloadArtifactExpired    ArtifactStatus = "Expired"
	DownloadArtifactCleanup    ArtifactStatus = "CleanupPending"

	DefaultDownloadMaxBytes int64 = 50 * 1024 * 1024
)

var (
	ErrDownloadArtifactNotFound = errors.New("download artifact not found")
	ErrDownloadArtifactExpired  = errors.New("download artifact has expired")
	ErrDownloadArtifactRevoked  = errors.New("download artifact has been revoked")
	ErrDownloadArtifactConsumed = errors.New("download artifact has already been downloaded")
	ErrDownloadArtifactInvalid  = errors.New("download artifact is invalid")
	ErrDownloadArtifactChecksum = errors.New("download artifact checksum mismatch")
)

type ArtifactStatus string

// DownloadArtifact is the persisted control record for a governed export.
// Its registry implementation must make ClaimDownload atomic across workers.
type DownloadArtifact struct {
	ID             string
	OrganizationID string
	ActorID        string
	ApprovalID     string
	ObjectKey      string
	ContentType    string
	SHA256         string
	Size           int64
	Status         ArtifactStatus
	MaxDownloads   int
	Downloads      int
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DownloadedAt   time.Time
	RevokedAt      time.Time
	RevokedReason  string
}

// ArtifactObjectStore deliberately matches the existing provider-neutral
// storage.Store surface. Metadata and retention policy remain in the record so
// local, generic S3, COS, OSS and OBS adapters share the same control plane.
type ArtifactObjectStore interface {
	Put(context.Context, string, io.Reader) error
	Get(context.Context, string) (io.ReadCloser, error)
	Delete(context.Context, string) error
}

// ArtifactRegistry is the persistence boundary. ClaimDownload must perform a
// conditional update in one transaction and return the post-claim record.
type ArtifactRegistry interface {
	Create(context.Context, DownloadArtifact) error
	Get(context.Context, string, string) (DownloadArtifact, error)
	ClaimDownload(context.Context, string, string, string, time.Time) (DownloadArtifact, error)
	Revoke(context.Context, string, string, string, string, time.Time) (DownloadArtifact, error)
	Expire(context.Context, string, string, time.Time) (DownloadArtifact, error)
	MarkCleanupPending(context.Context, string, string, time.Time) (DownloadArtifact, error)
	CompleteCleanup(context.Context, string, string, time.Time) (DownloadArtifact, error)
}

type DownloadController struct {
	registry ArtifactRegistry
	objects  ArtifactObjectStore
	maxBytes int64
	now      func() time.Time
}

func NewDownloadController(registry ArtifactRegistry, objects ArtifactObjectStore, maxBytes int64) (*DownloadController, error) {
	if registry == nil || objects == nil {
		return nil, errors.New("download artifact registry and object store are required")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultDownloadMaxBytes
	}
	return &DownloadController{
		registry: registry,
		objects:  objects,
		maxBytes: maxBytes,
		now:      func() time.Time { return time.Now().UTC() },
	}, nil
}

// Publish stores the bytes before publishing the control record. If the
// registry rejects the record, the object is deleted and no download ticket
// is exposed. Callers must bind ApprovalID to an already-authorized request.
func (c *DownloadController) Publish(ctx context.Context, organizationID, actorID, approvalID, contentType string, payload []byte, expiresAt time.Time) (DownloadArtifact, error) {
	organizationID = strings.TrimSpace(organizationID)
	actorID = strings.TrimSpace(actorID)
	approvalID = strings.TrimSpace(approvalID)
	contentType = strings.TrimSpace(contentType)
	now := c.now().UTC()
	if organizationID == "" || actorID == "" || approvalID == "" || contentType == "" || len(payload) == 0 || int64(len(payload)) > c.maxBytes || expiresAt.UTC().Before(now) || expiresAt.UTC().Equal(now) {
		return DownloadArtifact{}, ErrDownloadArtifactInvalid
	}
	digest := sha256.Sum256(payload)
	artifactID := uuid.NewString()
	key := "governed-exports/" + organizationID + "/" + artifactID
	if err := c.objects.Put(ctx, key, bytes.NewReader(payload)); err != nil {
		return DownloadArtifact{}, fmt.Errorf("store governed export: %w", err)
	}
	artifact := DownloadArtifact{
		ID: artifactID, OrganizationID: organizationID, ActorID: actorID, ApprovalID: approvalID,
		ObjectKey: key, ContentType: contentType, SHA256: hex.EncodeToString(digest[:]),
		Size: int64(len(payload)), Status: DownloadArtifactReady, MaxDownloads: 1,
		ExpiresAt: expiresAt.UTC(), CreatedAt: now, UpdatedAt: now,
	}
	if err := c.registry.Create(ctx, artifact); err != nil {
		_ = c.objects.Delete(ctx, key)
		return DownloadArtifact{}, fmt.Errorf("create governed export record: %w", err)
	}
	return artifact, nil
}

// Open validates and reads the object before claiming the ticket. This avoids
// exposing bytes when a concurrent worker already consumed the one-time
// ticket, while the registry claim still provides the atomic race boundary.
func (c *DownloadController) Open(ctx context.Context, organizationID, artifactID, actorID string) (DownloadArtifact, io.ReadCloser, error) {
	artifact, err := c.registry.Get(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(artifactID))
	if err != nil {
		return DownloadArtifact{}, nil, err
	}
	if artifact.Status == DownloadArtifactRevoked {
		return artifact, nil, ErrDownloadArtifactRevoked
	}
	if artifact.Status == DownloadArtifactCleanup {
		return artifact, nil, ErrDownloadArtifactInvalid
	}
	if artifact.Status == DownloadArtifactDownloaded || artifact.Downloads >= artifact.MaxDownloads {
		return artifact, nil, ErrDownloadArtifactConsumed
	}
	if c.now().UTC().After(artifact.ExpiresAt) {
		return artifact, nil, ErrDownloadArtifactExpired
	}
	if artifact.Size <= 0 || artifact.Size > c.maxBytes || len(artifact.SHA256) != sha256.Size*2 {
		return artifact, nil, ErrDownloadArtifactInvalid
	}
	body, err := c.objects.Get(ctx, artifact.ObjectKey)
	if err != nil {
		return artifact, nil, fmt.Errorf("read governed export: %w", err)
	}
	payload, readErr := io.ReadAll(io.LimitReader(body, c.maxBytes+1))
	closeErr := body.Close()
	if readErr != nil {
		return artifact, nil, fmt.Errorf("read governed export: %w", readErr)
	}
	if closeErr != nil {
		return artifact, nil, fmt.Errorf("close governed export: %w", closeErr)
	}
	if int64(len(payload)) != artifact.Size || int64(len(payload)) > c.maxBytes {
		return artifact, nil, ErrDownloadArtifactChecksum
	}
	digest := sha256.Sum256(payload)
	if !strings.EqualFold(hex.EncodeToString(digest[:]), artifact.SHA256) {
		return artifact, nil, ErrDownloadArtifactChecksum
	}
	claimed, err := c.registry.ClaimDownload(ctx, artifact.OrganizationID, artifact.ID, strings.TrimSpace(actorID), c.now().UTC())
	if err != nil {
		return artifact, nil, err
	}
	return claimed, io.NopCloser(bytes.NewReader(payload)), nil
}

// Revoke changes the database state before deleting the object. A deletion
// failure leaves a non-downloadable CleanupPending record for an operator or
// scheduled cleanup job to retry; it never reopens access to the artifact.
func (c *DownloadController) Revoke(ctx context.Context, organizationID, artifactID, actorID, reason string) error {
	artifact, err := c.RevokeState(ctx, organizationID, artifactID, actorID, reason)
	if err != nil {
		return err
	}
	return c.cleanupArtifact(ctx, artifact)
}

func (c *DownloadController) Expire(ctx context.Context, organizationID, artifactID string) error {
	artifact, err := c.ExpireState(ctx, organizationID, artifactID)
	if err != nil {
		return err
	}
	return c.cleanupArtifact(ctx, artifact)
}

func (c *DownloadController) RevokeState(ctx context.Context, organizationID, artifactID, actorID, reason string) (DownloadArtifact, error) {
	return c.registry.Revoke(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(artifactID), strings.TrimSpace(actorID), strings.TrimSpace(reason), c.now().UTC())
}

func (c *DownloadController) ExpireState(ctx context.Context, organizationID, artifactID string) (DownloadArtifact, error) {
	return c.registry.Expire(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(artifactID), c.now().UTC())
}

func (c *DownloadController) cleanupArtifact(ctx context.Context, artifact DownloadArtifact) error {
	if err := c.objects.Delete(ctx, artifact.ObjectKey); err != nil {
		if _, markErr := c.registry.MarkCleanupPending(ctx, artifact.OrganizationID, artifact.ID, c.now().UTC()); markErr != nil {
			return fmt.Errorf("delete export object: %w; mark cleanup pending: %v", err, markErr)
		}
		return fmt.Errorf("delete export object; cleanup pending: %w", err)
	}
	if artifact.Status == DownloadArtifactCleanup {
		if _, err := c.registry.CompleteCleanup(ctx, artifact.OrganizationID, artifact.ID, c.now().UTC()); err != nil {
			return fmt.Errorf("complete export cleanup: %w", err)
		}
	}
	return nil
}

// Cleanup retries an object deletion after Revoke or Expire failed. The
// database record remains non-downloadable throughout the retry.
func (c *DownloadController) Cleanup(ctx context.Context, organizationID, artifactID string) error {
	artifact, err := c.registry.Get(ctx, strings.TrimSpace(organizationID), strings.TrimSpace(artifactID))
	if err != nil {
		return err
	}
	if artifact.Status != DownloadArtifactCleanup && artifact.Status != DownloadArtifactRevoked && artifact.Status != DownloadArtifactExpired {
		return ErrDownloadArtifactInvalid
	}
	return c.cleanupArtifact(ctx, artifact)
}

// memoryArtifactRegistry is intentionally test-only. Production composition
// must use the SQL repository whose ClaimDownload is a conditional update.
type memoryArtifactRegistry struct {
	mu    sync.Mutex
	items map[string]DownloadArtifact
}

func newMemoryArtifactRegistry() *memoryArtifactRegistry {
	return &memoryArtifactRegistry{items: make(map[string]DownloadArtifact)}
}

func (r *memoryArtifactRegistry) Create(_ context.Context, artifact DownloadArtifact) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[artifact.ID]; exists {
		return errors.New("artifact already exists")
	}
	r.items[artifact.ID] = artifact
	return nil
}

func (r *memoryArtifactRegistry) Get(_ context.Context, organizationID, artifactID string) (DownloadArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	artifact, ok := r.items[artifactID]
	if !ok || artifact.OrganizationID != organizationID {
		return DownloadArtifact{}, ErrDownloadArtifactNotFound
	}
	return artifact, nil
}

func (r *memoryArtifactRegistry) ClaimDownload(_ context.Context, organizationID, artifactID, actorID string, now time.Time) (DownloadArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	artifact, ok := r.items[artifactID]
	if !ok || artifact.OrganizationID != organizationID {
		return DownloadArtifact{}, ErrDownloadArtifactNotFound
	}
	if artifact.Status == DownloadArtifactRevoked {
		return artifact, ErrDownloadArtifactRevoked
	}
	if artifact.Status == DownloadArtifactDownloaded || artifact.Downloads >= artifact.MaxDownloads {
		return artifact, ErrDownloadArtifactConsumed
	}
	if now.After(artifact.ExpiresAt) {
		artifact.Status = DownloadArtifactExpired
		r.items[artifactID] = artifact
		return artifact, ErrDownloadArtifactExpired
	}
	if actorID == "" || actorID != artifact.ActorID {
		return artifact, errors.New("download actor is not authorized")
	}
	artifact.Downloads++
	artifact.Status = DownloadArtifactDownloaded
	artifact.DownloadedAt = now.UTC()
	artifact.UpdatedAt = now.UTC()
	r.items[artifactID] = artifact
	return artifact, nil
}

func (r *memoryArtifactRegistry) Revoke(_ context.Context, organizationID, artifactID, actorID, reason string, now time.Time) (DownloadArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	artifact, ok := r.items[artifactID]
	if !ok || artifact.OrganizationID != organizationID {
		return DownloadArtifact{}, ErrDownloadArtifactNotFound
	}
	if actorID == "" || actorID != artifact.ActorID {
		return artifact, errors.New("download actor is not authorized")
	}
	if artifact.Status == DownloadArtifactDownloaded || artifact.Status == DownloadArtifactRevoked || artifact.Status == DownloadArtifactExpired {
		return artifact, ErrDownloadArtifactConsumed
	}
	artifact.Status = DownloadArtifactRevoked
	artifact.RevokedAt = now.UTC()
	artifact.RevokedReason = reason
	artifact.UpdatedAt = now.UTC()
	r.items[artifactID] = artifact
	return artifact, nil
}

func (r *memoryArtifactRegistry) Expire(_ context.Context, organizationID, artifactID string, now time.Time) (DownloadArtifact, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	artifact, ok := r.items[artifactID]
	if !ok || artifact.OrganizationID != organizationID {
		return DownloadArtifact{}, ErrDownloadArtifactNotFound
	}
	if artifact.Status != DownloadArtifactReady {
		return artifact, ErrDownloadArtifactConsumed
	}
	if now.Before(artifact.ExpiresAt) {
		return artifact, errors.New("download artifact has not expired")
	}
	artifact.Status = DownloadArtifactExpired
	artifact.UpdatedAt = now.UTC()
	r.items[artifactID] = artifact
	return artifact, nil
}
