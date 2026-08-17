package storage

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sevoniva-labs/forge/internal/platform/config"
)

func TestLocalStoreRejectsTraversalAndEscapingSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	store, err := New(context.Background(), config.Storage{Provider: "local", LocalRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"../outside", "/absolute", "escape/secret"} {
		if err := store.Put(context.Background(), key, strings.NewReader("secret")); err == nil {
			t.Fatalf("unsafe key %q was accepted", key)
		}
	}
	if _, err := os.Stat(filepath.Join(outside, "secret")); !os.IsNotExist(err) {
		t.Fatalf("write escaped local root: %v", err)
	}
}

func TestLocalStoreRoundTripUsesPrivateFileMode(t *testing.T) {
	root := t.TempDir()
	store, err := New(context.Background(), config.Storage{Provider: "local", LocalRoot: root})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := store.Put(ctx, "documents/report.txt", strings.NewReader("banking")); err != nil {
		t.Fatal(err)
	}
	r, err := store.Get(ctx, "documents/report.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = r.Close() }()
	got, err := io.ReadAll(r)
	if err != nil || string(got) != "banking" {
		t.Fatalf("round trip got %q, err %v", got, err)
	}
	info, err := os.Stat(filepath.Join(root, "documents", "report.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("file mode = %o, want 600", info.Mode().Perm())
	}
}
