//go:build integration

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lethe/lethe/internal/module"
	"github.com/lethe/lethe/internal/risk"
)

func TestShredFileGone(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "secret.txt"), "sensitive data that must be destroyed")

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
}

func TestShredDir(t *testing.T) {
	dir := t.TempDir()
	sub := mkdir(t, filepath.Join(dir, "subdir"))
	writeFileAbs(t, filepath.Join(sub, "file1.txt"), "data1")
	writeFileAbs(t, filepath.Join(sub, "file2.txt"), "data2")

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: sub, Method: "delete", Risk: "safe", Recursive: true}
	eng.cleanArtifact("test", a)

	assertFileGone(t, sub)
}

func TestShredEmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "empty.txt"), "")

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
}

func TestShredLargeFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "large.bin")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	if _, err := f.Write(content); err != nil {
		t.Fatal(err)
	}
	f.Close()

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
}

func TestShredMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 10; i++ {
		writeFileAbs(t, filepath.Join(dir, filepath.FromSlash(fmt.Sprintf("file%02d.txt", i))), "sensitive")
	}

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: filepath.Join(dir, "*.txt"), Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected all files shredded, got %d remaining", len(entries))
	}
}

func TestShredNonexistentFile(t *testing.T) {
	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: "/tmp/lethe-nonexistent-shred-test", Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	if eng.stats.Failed > 0 {
		t.Errorf("nonexistent file should not cause failure in shred path, got %d failed", eng.stats.Failed)
	}
}

func TestTruncateFileReal(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "history.log"), "user logged in at 2026-05-30\nuser ran rm -rf\n")

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "truncate", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileExists(t, path)
	assertFileEmpty(t, path)
}

func TestDeleteFileReal(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "cache.tmp"), "cached data")

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
}

func TestDeleteDirRecursive(t *testing.T) {
	dir := t.TempDir()
	sub := mkdir(t, filepath.Join(dir, "nested"))
	writeFileAbs(t, filepath.Join(sub, "a.txt"), "a")
	writeFileAbs(t, filepath.Join(sub, "b.txt"), "b")

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: sub, Method: "delete", Risk: "safe", Recursive: true}
	eng.cleanArtifact("test", a)

	assertFileGone(t, sub)
}

func TestDryRunPreservesFiles(t *testing.T) {
	dir := t.TempDir()
	content := "important data that must survive dry-run"
	path := writeFileAbs(t, filepath.Join(dir, "keep.txt"), content)

	eng, _ := newTestEngine(t)
	eng.dryRun = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileExists(t, path)
	assertFileContent(t, path, content)
	if eng.stats.Cleaned != 1 {
		t.Errorf("dry-run should count as cleaned, got %d", eng.stats.Cleaned)
	}
}

func TestRiskPolicyBlocksDestructive(t *testing.T) {
	homeDir := t.TempDir()
	content := "destructive data"
	path := writeFileAbs(t, filepath.Join(homeDir, "destruct.txt"), content)

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: path, Method: "delete", Risk: "destructive"},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskDestructive, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	if err := eng.Run(RunOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileContent(t, path, content)
}

func TestExcludePatternReal(t *testing.T) {
	dir := t.TempDir()
	keepPath := writeFileAbs(t, filepath.Join(dir, "important.log"), "keep")
	junkPath := writeFileAbs(t, filepath.Join(dir, "junk.log"), "junk")

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: filepath.Join(dir, "*.log"), Method: "delete", Risk: "safe", Exclude: []string{"important.log"}}
	eng.cleanArtifact("test", a)

	assertFileExists(t, keepPath)
	assertFileGone(t, junkPath)
}

func TestShredOriginalNameGone(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "evidence.txt"), "sensitive")

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
	leftover, _ := filepath.Glob(filepath.Join(dir, "lethe-*"))
	if len(leftover) != 0 {
		t.Errorf("temp shred files left behind: %v", leftover)
	}
}

func TestShredDirOriginalNameGone(t *testing.T) {
	dir := t.TempDir()
	sub := mkdir(t, filepath.Join(dir, "sensitive_dir"))
	writeFileAbs(t, filepath.Join(sub, "file.txt"), "data")

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: sub, Method: "delete", Risk: "safe", Recursive: true}
	eng.cleanArtifact("test", a)

	assertFileGone(t, sub)
	leftover, _ := filepath.Glob(filepath.Join(dir, "lethe-*"))
	if len(leftover) != 0 {
		t.Errorf("temp shred dirs left behind: %v", leftover)
	}
}

func TestShredRenameFallback(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "root_owned.txt"), "sensitive")

	origRename := osRename
	osRename = func(oldpath, newpath string) error {
		return fmt.Errorf("EXDEV: cross-device link")
	}
	defer func() { osRename = origRename }()

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
}
