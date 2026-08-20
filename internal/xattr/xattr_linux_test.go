package xattr

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestClearRemovesXattrs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := unix.Setxattr(path, "user.testattr", []byte("value"), 0); err != nil {
		t.Skipf("filesystem does not support xattr: %v", err)
	}

	if err := Clear(path); err != nil {
		t.Fatalf("Clear: %v", err)
	}

	_, err := unix.Llistxattr(path, nil)
	if err != nil {
		t.Fatalf("list xattr: %v", err)
	}
	buf := make([]byte, 1024)
	n, err := unix.Llistxattr(path, buf)
	if err != nil {
		t.Fatalf("list xattr: %v", err)
	}
	if n != 0 {
		t.Errorf("expected no xattrs after clear, got %d bytes", n)
	}
}

func TestClearNonexistentPath(t *testing.T) {
	if err := Clear(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected error for nonexistent path")
	}
}

func TestClearNoXattrs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "clean.txt")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := Clear(path); err != nil {
		t.Errorf("clear with no xattrs should succeed: %v", err)
	}
}
