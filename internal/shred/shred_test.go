package shred

import (
	"os"
	"path/filepath"
	"testing"
)

func TestShredFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "testfile")
	content := []byte("sensitive data that should be destroyed")
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := ShredFile(path, 3); err != nil {
		t.Fatalf("ShredFile failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after shred")
	}
}

func TestShredFileNonexistent(t *testing.T) {
	err := ShredFile("/nonexistent/path/file.txt", 3)
	if err != nil {
		t.Errorf("shredding nonexistent file should not error, got: %v", err)
	}
}

func TestShredFileEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}

	if err := ShredFile(path, 3); err != nil {
		t.Fatalf("ShredFile failed on empty: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("empty file should be removed after shred")
	}
}

func TestShredDirectory(t *testing.T) {
	dir := t.TempDir()
	err := ShredFile(dir, 3)
	if err != nil {
		t.Errorf("shredding directory should not error, got: %v", err)
	}

	if _, err := os.Stat(dir); err != nil {
		t.Error("directory should still exist after shred attempt")
	}
}

func TestShredLargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "largefile")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(make([]byte, 100*1024)); err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := ShredFile(path, 3); err != nil {
		t.Fatalf("ShredFile failed on large file: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("large file should not exist after shred")
	}
}
