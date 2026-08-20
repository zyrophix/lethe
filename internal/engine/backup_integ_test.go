//go:build integration

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lethe/lethe/internal/module"
)

func TestBackupRestoreFullCycle(t *testing.T) {
	srcDir := t.TempDir()
	backupBase := t.TempDir()

	file1 := writeFileAbs(t, filepath.Join(srcDir, "shell.log"), "user commands history")
	file2 := writeFileAbs(t, filepath.Join(srcDir, "audit.log"), "audit trail data")

	b := NewBackup(backupBase)
	artifacts := []module.Artifact{
		{Path: file1, Backup: true},
		{Path: file2, Backup: true},
	}

	if err := b.Create(artifacts, ""); err != nil {
		t.Fatalf("backup create: %v", err)
	}

	assertFileExists(t, b.Path())

	os.Remove(file1)
	os.Remove(file2)
	assertFileGone(t, file1)
	assertFileGone(t, file2)

	if err := b.Restore(); err != nil {
		t.Fatalf("backup restore: %v", err)
	}

	assertFileContent(t, file1, "user commands history")
	assertFileContent(t, file2, "audit trail data")

	b.Cleanup()
	assertFileGone(t, b.Path())
}

func TestBackupRestoreWithDirs(t *testing.T) {
	srcDir := t.TempDir()
	backupBase := t.TempDir()

	sub := mkdir(t, filepath.Join(srcDir, "nested"))
	writeFileAbs(t, filepath.Join(sub, "deep.log"), "nested content")

	b := NewBackup(backupBase)
	artifacts := []module.Artifact{
		{Path: sub, Backup: true},
	}

	if err := b.Create(artifacts, ""); err != nil {
		t.Fatalf("backup create with dir: %v", err)
	}

	os.RemoveAll(sub)
	assertFileGone(t, sub)

	if err := b.Restore(); err != nil {
		t.Fatalf("backup restore with dir: %v", err)
	}

	assertFileContent(t, filepath.Join(sub, "deep.log"), "nested content")
}

func TestBackupSkipsNonBackupArtifacts(t *testing.T) {
	dir := t.TempDir()
	backupBase := t.TempDir()

	path := writeFileAbs(t, filepath.Join(dir, "no-backup.log"), "data")

	b := NewBackup(backupBase)
	artifacts := []module.Artifact{
		{Path: path, Backup: false},
	}

	if err := b.Create(artifacts, ""); err != nil {
		t.Fatalf("backup create: %v", err)
	}

	paths := b.collectExistingPaths(artifacts, "")
	if len(paths) != 0 {
		t.Errorf("backup=false artifacts should not be collected, got %d", len(paths))
	}
}

func TestBackupWithHomeDirResolution(t *testing.T) {
	homeDir := t.TempDir()
	backupBase := t.TempDir()

	writeFileAbs(t, filepath.Join(homeDir, ".bash_history"), "secret commands")

	b := NewBackup(backupBase)
	artifacts := []module.Artifact{
		{Path: "{{.HomeDir}}/.bash_history", Backup: true},
	}

	if err := b.Create(artifacts, homeDir); err != nil {
		t.Fatalf("backup create with template: %v", err)
	}

	histPath := filepath.Join(homeDir, ".bash_history")
	os.Remove(histPath)
	assertFileGone(t, histPath)

	if err := b.Restore(); err != nil {
		t.Fatalf("backup restore: %v", err)
	}

	assertFileContent(t, histPath, "secret commands")
}

func TestBackupCleanupRemovesTar(t *testing.T) {
	dir := t.TempDir()
	backupBase := t.TempDir()

	path := writeFileAbs(t, filepath.Join(dir, "file.txt"), "data")

	b := NewBackup(backupBase)
	artifacts := []module.Artifact{{Path: path, Backup: true}}

	b.Create(artifacts, "")
	assertFileExists(t, b.Path())

	b.Cleanup()
	assertFileGone(t, b.Path())
}

func TestBackupEmptyArtifactsNoError(t *testing.T) {
	backupBase := t.TempDir()

	b := NewBackup(backupBase)
	artifacts := []module.Artifact{
		{Path: "/tmp/lethe-nonexistent-backup-test", Backup: true},
	}

	if err := b.Create(artifacts, ""); err != nil {
		t.Errorf("empty backup should not error: %v", err)
	}
}
