package datapolicy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

type cleanupTestStore struct {
	payload     []byte
	failDeletes bool
	deleted     bool
	gets        int
}

func (s *cleanupTestStore) Put(_ context.Context, _ string, body io.Reader) error {
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	s.payload = append([]byte(nil), payload...)
	return nil
}

func (s *cleanupTestStore) Get(_ context.Context, _ string) (io.ReadCloser, error) {
	s.gets++
	return io.NopCloser(bytes.NewReader(s.payload)), nil
}

func (s *cleanupTestStore) Delete(_ context.Context, _ string) error {
	if s.failDeletes {
		return errors.New("object store unavailable")
	}
	s.deleted = true
	s.payload = nil
	return nil
}

func TestDownloadRejectsWrongActorBeforeReadingObject(t *testing.T) {
	registry := newMemoryArtifactRegistry()
	objects := &cleanupTestStore{}
	controller, err := NewDownloadController(registry, objects, 1024)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := controller.Publish(context.Background(), "org-1", "user-1", "approval-1", "text/csv", []byte("id,name\n1,Alice\n"), time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-2"); !errors.Is(err, ErrDownloadArtifactActor) {
		t.Fatalf("expected actor mismatch, got %v", err)
	}
	if objects.gets != 0 {
		t.Fatalf("wrong actor triggered object read: %d", objects.gets)
	}
}

func TestDownloadDeletesObjectAndRetriesWhenStorageIsUnavailable(t *testing.T) {
	registry := newMemoryArtifactRegistry()
	objects := &cleanupTestStore{failDeletes: true}
	controller, err := NewDownloadController(registry, objects, 1024)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := controller.Publish(context.Background(), "org-1", "user-1", "approval-1", "text/csv", []byte("id,name\n1,Alice\n"), time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	_, body, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(body); err != nil {
		t.Fatal(err)
	}
	pending, err := registry.Get(context.Background(), "org-1", artifact.ID)
	if err != nil || pending.Status != DownloadArtifactCleanup {
		t.Fatalf("expected downloaded object cleanup pending, got %+v, %v", pending, err)
	}
	objects.failDeletes = false
	if err := controller.Cleanup(context.Background(), "org-1", artifact.ID); err != nil {
		t.Fatal(err)
	}
	closed, err := registry.Get(context.Background(), "org-1", artifact.ID)
	if err != nil || closed.Status != DownloadArtifactDownloaded || !objects.deleted {
		t.Fatalf("expected consumed artifact after cleanup, got %+v, deleted=%v, err=%v", closed, objects.deleted, err)
	}
}

func TestExpirePendingClosesAndDeletesExpiredTicket(t *testing.T) {
	registry := newMemoryArtifactRegistry()
	objects := &cleanupTestStore{}
	controller, err := NewDownloadController(registry, objects, 1024)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := controller.Publish(context.Background(), "org-1", "user-1", "approval-1", "text/csv", []byte("id,name\n1,Alice\n"), time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	controller.now = func() time.Time { return time.Now().UTC().Add(2 * time.Hour) }
	count, err := controller.ExpirePending(context.Background(), 10)
	if err != nil || count != 1 {
		t.Fatalf("expected one expired ticket, got count=%d err=%v", count, err)
	}
	closed, err := registry.Get(context.Background(), "org-1", artifact.ID)
	if err != nil || closed.Status != DownloadArtifactExpired || !objects.deleted {
		t.Fatalf("expected expired artifact and deleted object, got %+v, deleted=%v, err=%v", closed, objects.deleted, err)
	}
}

func TestDownloadCleanupClosesAccessBeforeRetryingObjectDeletion(t *testing.T) {
	registry := newMemoryArtifactRegistry()
	objects := &cleanupTestStore{}
	controller, err := NewDownloadController(registry, objects, 1024)
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := controller.Publish(context.Background(), "org-1", "user-1", "approval-1", "text/csv", []byte("id,name\n1,Alice\n"), time.Now().UTC().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	objects.failDeletes = true
	if err := controller.Revoke(context.Background(), "org-1", artifact.ID, "user-1", "operator request"); err == nil {
		t.Fatal("expected cleanup-pending error")
	}
	pending, err := registry.Get(context.Background(), "org-1", artifact.ID)
	if err != nil || pending.Status != DownloadArtifactCleanup {
		t.Fatalf("expected cleanup pending artifact, got %+v, %v", pending, err)
	}
	if _, _, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-1"); !errors.Is(err, ErrDownloadArtifactInvalid) {
		t.Fatalf("expected pending artifact to remain closed, got %v", err)
	}
	objects.failDeletes = false
	if err := controller.Cleanup(context.Background(), "org-1", artifact.ID); err != nil {
		t.Fatal(err)
	}
	closed, err := registry.Get(context.Background(), "org-1", artifact.ID)
	if err != nil || closed.Status != DownloadArtifactRevoked || !objects.deleted {
		t.Fatalf("expected revoked artifact after cleanup, got %+v, deleted=%v, err=%v", closed, objects.deleted, err)
	}
}
