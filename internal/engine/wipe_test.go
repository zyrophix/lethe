package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestWipeFreeSpaceCreatesAndRemovesFiller(t *testing.T) {
	dir := t.TempDir()

	written, err := (&Engine{}).wipeFreeSpace(context.Background(), dir, 256*1024)
	if err != nil {
		t.Fatalf("wipeFreeSpace: %v", err)
	}
	if written <= 0 {
		t.Errorf("expected some bytes written, got %d", written)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("filler file should be removed, got %d entries", len(entries))
	}
}

func TestWipeFreeSpaceRespectsCap(t *testing.T) {
	dir := t.TempDir()

	const capBytes = 1024 * 1024
	written, err := (&Engine{}).wipeFreeSpace(context.Background(), dir, capBytes)
	if err != nil {
		t.Fatalf("wipeFreeSpace: %v", err)
	}
	if written != capBytes {
		t.Errorf("expected %d bytes written, got %d", capBytes, written)
	}
}

func TestWipeFreeSpaceMissingDir(t *testing.T) {
	_, err := (&Engine{}).wipeFreeSpace(context.Background(), filepath.Join(t.TempDir(), "does-not-exist"), -1)
	if err == nil {
		t.Fatal("expected error for missing dir")
	}
}
