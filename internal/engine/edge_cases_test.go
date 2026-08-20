//go:build integration

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/lethe/lethe/internal/module"
)

func TestFileWithSpaces(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "file with spaces.log"), "space content")

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
}

func TestFileWithUnicode(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "журнал.log"), "unicode content")

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
}

func TestSymlinkInDir(t *testing.T) {
	dir := t.TempDir()
	target := writeFileAbs(t, filepath.Join(dir, "real.txt"), "real content")
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Skip("symlinks not supported")
	}

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: filepath.Join(dir, "*.txt"), Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, target)
	assertFileGone(t, link)
}

func TestReadOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "readonly.log"), "readonly content")
	os.Chmod(path, 0444)

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileGone(t, path)
	os.Chmod(path, 0644)
}

func TestEmptyDirectoryDelete(t *testing.T) {
	dir := t.TempDir()
	emptyDir := mkdir(t, filepath.Join(dir, "empty"))

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: emptyDir, Method: "delete", Risk: "safe", Recursive: true}
	eng.cleanArtifact("test", a)

	assertFileGone(t, emptyDir)
}

func TestGlobPatternNoMatches(t *testing.T) {
	dir := t.TempDir()

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: filepath.Join(dir, "*.nonexistent"), Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	if eng.stats.Cleaned != 0 {
		t.Errorf("no matches should mean 0 cleaned, got %d", eng.stats.Cleaned)
	}
}

func TestMultipleGlobMatches(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		writeFileAbs(t, filepath.Join(dir, filepath.FromSlash(fmt.Sprintf("log%02d.txt", i))), "data")
	}

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: filepath.Join(dir, "*.txt"), Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected all files deleted, got %d remaining", len(entries))
	}
	if eng.stats.Cleaned != 5 {
		t.Errorf("expected 5 cleaned, got %d", eng.stats.Cleaned)
	}
}

func TestTruncatePreservesExistence(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "big.log"), "lots of log data here")

	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "truncate", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileExists(t, path)
	assertFileEmpty(t, path)

	info, _ := os.Stat(path)
	if info.IsDir() {
		t.Error("truncated file should not become a directory")
	}
}

func TestUnresolvedPathSkipped(t *testing.T) {
	eng, _ := newTestEngine(t)
	eng.dryRun = false

	a := module.Artifact{Path: "{{.NonexistentVar}}/file.txt", Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	if eng.stats.Cleaned != 0 {
		t.Errorf("unresolved path should be skipped, got %d cleaned", eng.stats.Cleaned)
	}
}
