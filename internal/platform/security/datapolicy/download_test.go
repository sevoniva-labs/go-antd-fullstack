package datapolicy

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"
)

type memoryArtifactObjects struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func newMemoryArtifactObjects() *memoryArtifactObjects {
	return &memoryArtifactObjects{objects: make(map[string][]byte)}
}

func (s *memoryArtifactObjects) Put(_ context.Context, key string, reader io.Reader) error {
	payload, err := io.ReadAll(reader)
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.objects[key] = append([]byte(nil), payload...)
	s.mu.Unlock()
	return nil
}

func (s *memoryArtifactObjects) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.Lock()
	payload, ok := s.objects[key]
	s.mu.Unlock()
	if !ok {
		return nil, errors.New("object not found")
	}
	return io.NopCloser(bytes.NewReader(payload)), nil
}

func (s *memoryArtifactObjects) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	delete(s.objects, key)
	s.mu.Unlock()
	return nil
}

func TestDownloadControllerPublishesAndConsumesOnce(t *testing.T) {
	registry := newMemoryArtifactRegistry()
	objects := newMemoryArtifactObjects()
	controller, err := NewDownloadController(registry, objects, 1024)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	controller.now = func() time.Time { return now }
	artifact, err := controller.Publish(context.Background(), "org-1", "user-1", "approval-1", "text/csv", []byte("id,name\n1,Alice\n"), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	claimed, body, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-1")
	if err != nil {
		t.Fatal(err)
	}
	payload, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	_ = body.Close()
	if string(payload) != "id,name\n1,Alice\n" || claimed.Status != DownloadArtifactDownloaded {
		t.Fatalf("unexpected download: status=%s payload=%q", claimed.Status, payload)
	}
	if _, _, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-1"); !errors.Is(err, ErrDownloadArtifactConsumed) {
		t.Fatalf("expected one-time download rejection, got %v", err)
	}
}

func TestDownloadControllerFailsClosedForWrongActorAndCorruptObject(t *testing.T) {
	registry := newMemoryArtifactRegistry()
	objects := newMemoryArtifactObjects()
	controller, err := NewDownloadController(registry, objects, 1024)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	controller.now = func() time.Time { return now }
	artifact, err := controller.Publish(context.Background(), "org-1", "user-1", "approval-1", "application/json", []byte(`{"ok":true}`), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-2"); err == nil {
		t.Fatal("expected wrong actor rejection")
	}
	objects.mu.Lock()
	objects.objects[artifact.ObjectKey] = []byte(`{"ok":false}`)
	objects.mu.Unlock()
	if _, _, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-1"); !errors.Is(err, ErrDownloadArtifactChecksum) {
		t.Fatalf("expected checksum rejection, got %v", err)
	}
}

func TestDownloadControllerRevokesBeforeObjectCleanup(t *testing.T) {
	registry := newMemoryArtifactRegistry()
	objects := newMemoryArtifactObjects()
	controller, err := NewDownloadController(registry, objects, 1024)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	controller.now = func() time.Time { return now }
	artifact, err := controller.Publish(context.Background(), "org-1", "user-1", "approval-1", "application/json", []byte(`{"ok":true}`), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if err := controller.Revoke(context.Background(), "org-1", artifact.ID, "user-1", "operator request"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := controller.Open(context.Background(), "org-1", artifact.ID, "user-1"); !errors.Is(err, ErrDownloadArtifactRevoked) {
		t.Fatalf("expected revoked rejection, got %v", err)
	}
}
