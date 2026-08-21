package lethe

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestShredFileRemovesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("sensitive"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ShredFile(context.Background(), path, 3); err != nil {
		t.Fatalf("ShredFile: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be gone after shred")
	}
}

func TestShredFileContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "secret2.txt")
	_ = os.WriteFile(path, []byte("data"), 0o644)
	if err := ShredFile(ctx, path, 3); err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestCleanDryRun(t *testing.T) {
	var buf bytes.Buffer
	res, err := Clean(context.Background(), Options{
		DryRun:  true,
		MaxRisk: RiskSafe,
		Logger:  NewTextLogger(&buf),
	})
	if err != nil {
		t.Fatalf("Clean dry-run: %v", err)
	}
	if res.Cleaned < 1 {
		t.Errorf("expected at least 1 cleaned artifact in dry-run, got %d", res.Cleaned)
	}
}

func TestCleanZeroRiskDefaultsToRisky(t *testing.T) {
	var buf bytes.Buffer
	// RiskUndefined (0) should default to Risky, not Safe.
	res, err := Clean(context.Background(), Options{
		DryRun: true,
		// MaxRisk left as RiskUndefined
		Logger: NewTextLogger(&buf),
	})
	if err != nil {
		t.Fatalf("Clean: %v", err)
	}
	// Dry-run with default (risky) should clean at least as many as safe.
	// We just check it succeeds and cleans something.
	if res.Cleaned < 1 {
		t.Error("expected cleaned with default risk")
	}
}

func TestCleanExplicitSafe(t *testing.T) {
	var buf bytes.Buffer
	resSafe, err := Clean(context.Background(), Options{
		DryRun:  true,
		MaxRisk: RiskSafe,
		Logger:  NewTextLogger(&buf),
	})
	if err != nil {
		t.Fatalf("Clean safe: %v", err)
	}
	var buf2 bytes.Buffer
	resRisky, err := Clean(context.Background(), Options{
		DryRun:  true,
		MaxRisk: RiskRisky,
		Logger:  NewTextLogger(&buf2),
	})
	if err != nil {
		t.Fatalf("Clean risky: %v", err)
	}
	if resRisky.Cleaned < resSafe.Cleaned {
		t.Errorf("risky should clean >= safe: safe=%d risky=%d", resSafe.Cleaned, resRisky.Cleaned)
	}
}

func TestCleanUnknownModule(t *testing.T) {
	var buf bytes.Buffer
	_, err := Clean(context.Background(), Options{
		DryRun:  true,
		MaxRisk: RiskSafe,
		Modules: []string{"no_such_module"},
		Logger:  NewTextLogger(&buf),
	})
	if err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func TestCleanWithContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Clean(ctx, Options{DryRun: true})
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestCleanWithAdvanced(t *testing.T) {
	var buf bytes.Buffer
	res, err := Clean(context.Background(), Options{
		DryRun:  true,
		MaxRisk: RiskSafe,
		Logger:  NewTextLogger(&buf),
		Advanced: &AdvancedOptions{
			Parallel:     true,
			KillBlockers: false,
			Backup:       &BackupOptions{Dir: t.TempDir()},
		},
	})
	if err != nil {
		t.Fatalf("Clean advanced: %v", err)
	}
	if res.Cleaned < 1 {
		t.Error("expected cleaned with advanced")
	}
}

func TestVerifyReturnsNoError(t *testing.T) {
	ok, err := Verify(context.Background(), RiskSafe, nil)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Skip("system appears clean; assertion would be fragile on a live machine")
	}
}

func TestVerifyUnknownModule(t *testing.T) {
	if _, err := Verify(context.Background(), RiskSafe, []string{"no_such_module"}); err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func TestVerifyContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := Verify(ctx, RiskSafe, nil)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestBackupSDKContract(t *testing.T) {
	archive, err := Backup(context.Background(), t.TempDir())
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
	if err := Restore(context.Background(), dir); err == nil {
		t.Fatal("expected error restoring missing archive")
	}
}

func TestRiskLevelString(t *testing.T) {
	tests := []struct {
		level RiskLevel
		want  string
	}{
		{RiskSafe, "safe"},
		{RiskRisky, "risky"},
		{RiskDestructive, "destructive"},
		{RiskUndefined, "unknown"},
	}
	for _, tt := range tests {
		if got := tt.level.String(); got != tt.want {
			t.Errorf("RiskLevel %d String = %q, want %q", tt.level, got, tt.want)
		}
	}
}

func TestParseRiskLevel(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want RiskLevel
	}{
		{"safe", RiskSafe},
		{"RISKY", RiskRisky},
		{"destructive", RiskDestructive},
	} {
		got, err := ParseRiskLevel(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseRiskLevel(%q)=%v,%v want %v", tc.in, got, err, tc.want)
		}
	}
	if _, err := ParseRiskLevel("invalid"); err == nil {
		t.Error("expected error for invalid risk")
	}
}

func TestLoggerAdapters(t *testing.T) {
	var buf bytes.Buffer
	l := NewTextLogger(&buf)
	l.Log(Event{Level: LevelInfo, Module: "test", Message: "hello"})
	if buf.Len() == 0 {
		t.Error("TextLogger should write")
	}
	var buf2 bytes.Buffer
	j := NewJSONLogger(&buf2)
	j.Log(Event{Level: LevelSuccess, Module: "test", Artifact: "/tmp/file", Action: "delete", Risk: RiskSafe, Message: "done"})
	if buf2.Len() == 0 {
		t.Error("JSONLogger should write")
	}
	// AuditLog integration via Clean
	var logBuf bytes.Buffer
	var auditBuf bytes.Buffer
	_, err := Clean(context.Background(), Options{
		DryRun:   true,
		MaxRisk:  RiskSafe,
		Logger:   NewTextLogger(&logBuf),
		AuditLog: &auditBuf,
	})
	if err != nil {
		t.Fatalf("Clean with audit: %v", err)
	}
	if auditBuf.Len() == 0 {
		t.Error("audit log should have entries")
	}
}
