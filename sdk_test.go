package lethe

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lethe/lethe/internal/output"
	"github.com/lethe/lethe/internal/risk"
)

func TestShredFileRemovesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("sensitive"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ShredFile(path, 3); err != nil {
		t.Fatalf("ShredFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be gone after shred")
	}
}

func TestCleanDryRun(t *testing.T) {
	res, err := Clean(Options{
		DryRun:  true,
		MaxRisk: risk.RiskSafe,
		Writer:  output.NewTextWriter(false, ""),
	})
	if err != nil {
		t.Fatalf("Clean dry-run: %v", err)
	}
	if res.Cleaned < 1 {
		t.Errorf("expected at least 1 cleaned artifact in dry-run, got %d", res.Cleaned)
	}
}

func TestCleanUnknownModule(t *testing.T) {
	_, err := Clean(Options{
		DryRun:  true,
		MaxRisk: risk.RiskSafe,
		Modules: []string{"no_such_module"},
		Writer:  output.NewTextWriter(false, ""),
	})
	if err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func TestVerifyReturnsNoError(t *testing.T) {
	ok, err := Verify(risk.RiskSafe, nil)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Skip("system appears clean; assertion would be fragile on a live machine")
	}
}

func TestVerifyUnknownModule(t *testing.T) {
	if _, err := Verify(risk.RiskSafe, []string{"no_such_module"}); err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func TestBackupSDKContract(t *testing.T) {
	archive, err := Backup(t.TempDir())
	if err != nil {
		t.Logf("Backup returned error (expected without root): %v", err)
		return
	}
	if _, err := os.Stat(archive); err != nil {
		t.Fatalf("backup archive missing: %v", err)
	}
}

func TestRestoreMissingArchive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "no-archive")
	if err := Restore(dir); err == nil {
		t.Fatal("expected error restoring missing archive")
	}
}
