package engine

import (
	"archive/tar"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lethe/lethe/internal/module"
)

func TestBackupCreateAndRestore(t *testing.T) {
	dir := t.TempDir()

	srcDir := filepath.Join(dir, "src")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("hello"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("world"), 0644)

	backupDir := filepath.Join(dir, "backups")
	os.MkdirAll(backupDir, 0755)

	b := NewBackup(backupDir)
	artifacts := []module.Artifact{
		{Path: filepath.Join(srcDir, "file1.txt"), Backup: true},
		{Path: filepath.Join(srcDir, "file2.txt"), Backup: true},
	}

	if err := b.Create(artifacts, ""); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := os.Stat(b.Path()); os.IsNotExist(err) {
		t.Fatal("backup tar should exist")
	}

	os.Remove(filepath.Join(srcDir, "file1.txt"))
	os.Remove(filepath.Join(srcDir, "file2.txt"))

	if _, err := os.Stat(filepath.Join(srcDir, "file1.txt")); !os.IsNotExist(err) {
		t.Fatal("file1 should be removed before restore")
	}

	if err := b.Restore(); err != nil {
		t.Fatalf("restore: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(srcDir, "file1.txt"))
	if err != nil {
		t.Fatalf("read restored file1: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("file1 content: got %q, want %q", data, "hello")
	}

	data, err = os.ReadFile(filepath.Join(srcDir, "file2.txt"))
	if err != nil {
		t.Fatalf("read restored file2: %v", err)
	}
	if string(data) != "world" {
		t.Errorf("file2 content: got %q, want %q", data, "world")
	}

	b.Cleanup()
	if _, err := os.Stat(b.Path()); !os.IsNotExist(err) {
		t.Error("tar should be removed after cleanup")
	}
}

func TestBackupPathTraversalRejection(t *testing.T) {
	dir := t.TempDir()

	tarPath := filepath.Join(dir, "evil.tar")
	f, err := os.Create(tarPath)
	if err != nil {
		t.Fatal(err)
	}

	tw := tar.NewWriter(f)
	tw.WriteHeader(&tar.Header{
		Name:     "../../etc/passwd",
		Typeflag: tar.TypeReg,
		Mode:     0644,
		Size:     4,
	})
	tw.Write([]byte("evil"))
	tw.Close()
	f.Close()

	b := &Backup{Dir: tarPath[:len(tarPath)-4]}
	b.Create(nil, "")

	f, _ = os.Open(tarPath)
	tr := tar.NewReader(f)
	var restoreErr error
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		_, err = sanitizeRestorePath(hdr.Name)
		if err != nil {
			restoreErr = err
			break
		}
	}
	f.Close()

	if restoreErr == nil {
		t.Error("path traversal should be rejected")
	}
}

func TestBackupEmptyArtifacts(t *testing.T) {
	dir := t.TempDir()
	b := NewBackup(dir)

	artifacts := []module.Artifact{
		{Path: "/nonexistent/path/file.txt", Backup: true},
	}

	if err := b.Create(artifacts, ""); err != nil {
		t.Errorf("should succeed with no existing files: %v", err)
	}
}

func TestSanitizeRestorePath(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid home", "/home/user/file.txt", false},
		{"valid tmp", "/tmp/file.txt", false},
		{"valid var", "/var/log/syslog", false},
		{"traversal", "../../etc/passwd", true},
		{"outside prefix", "/opt/data", true},
		{"single dotdot", "/home/../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sanitizeRestorePath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("sanitizeRestorePath(%q): err=%v, wantErr=%v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestBackupNoBackupFlag(t *testing.T) {
	dir := t.TempDir()
	b := NewBackup(dir)

	artifacts := []module.Artifact{
		{Path: "/tmp/somefile", Backup: false},
	}

	paths := b.collectExistingPaths(artifacts, "")
	if len(paths) != 0 {
		t.Error("non-backup artifacts should be skipped")
	}
}

func TestBackupDefaultDir(t *testing.T) {
	b := NewBackup("")
	if b.Dir == "" {
		t.Error("default backup dir should not be empty")
	}
	if !strings.Contains(filepath.Base(b.Dir), "lethe-backup") {
		t.Errorf("unexpected default backup dir name: %s", b.Dir)
	}
}
