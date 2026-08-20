package securefile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadReturnsFileContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "evidence.json")
	if err := os.WriteFile(path, []byte(`{"status":"Target-tested"}`), 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("read test file: %v", err)
	}
	if string(got) != `{"status":"Target-tested"}` {
		t.Fatalf("unexpected content: %q", got)
	}
}

func TestReadRejectsFinalSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	link := filepath.Join(root, "evidence.json")
	if err := os.WriteFile(target, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write target file: %v", err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	if _, err := Read(link); err == nil {
		t.Fatal("expected final symlink to be rejected")
	}
}

func TestReadRejectsEmptyPath(t *testing.T) {
	for _, path := range []string{"", " ", string(filepath.Separator)} {
		if _, err := Read(path); err == nil {
			t.Fatalf("expected empty path %q to be rejected", path)
		}
	}
}
