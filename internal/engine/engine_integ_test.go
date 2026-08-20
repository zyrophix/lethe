//go:build integration

package engine

import (
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/risk"
)

func TestEngineRunWithMockModule(t *testing.T) {
	homeDir := t.TempDir()
	logPath := writeFileAbs(t, filepath.Join(homeDir, "app.log"), "log entries")

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: logPath, Method: "truncate", Risk: "safe"},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskSafe, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	if err := eng.Run(RunOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileEmpty(t, logPath)
	if eng.GetStats().Cleaned < 1 {
		t.Errorf("expected at least 1 cleaned, got %d", eng.GetStats().Cleaned)
	}
}

func TestEngineRunDryRunNoChanges(t *testing.T) {
	homeDir := t.TempDir()
	content := "must survive dry-run"
	logPath := writeFileAbs(t, filepath.Join(homeDir, "history.log"), content)

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: logPath, Method: "truncate", Risk: "safe"},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskSafe, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	if err := eng.Run(RunOptions{DryRun: true}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileContent(t, logPath, content)
}

func TestEngineRunRiskFiltering(t *testing.T) {
	homeDir := t.TempDir()
	safePath := writeFileAbs(t, filepath.Join(homeDir, "safe.log"), "safe")
	riskyPath := writeFileAbs(t, filepath.Join(homeDir, "risky.log"), "risky")
	destructivePath := writeFileAbs(t, filepath.Join(homeDir, "destructive.log"), "destructive")

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: safePath, Method: "truncate", Risk: "safe"},
		{Path: riskyPath, Method: "truncate", Risk: "risky"},
		{Path: destructivePath, Method: "delete", Risk: "destructive"},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskSafe, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	if err := eng.Run(RunOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileEmpty(t, safePath)
	assertFileContent(t, riskyPath, "risky")
	assertFileContent(t, destructivePath, "destructive")
}

func TestEngineRunModuleFilter(t *testing.T) {
	homeDir := t.TempDir()
	path1 := writeFileAbs(t, filepath.Join(homeDir, "mod1.log"), "data1")
	path2 := writeFileAbs(t, filepath.Join(homeDir, "mod2.log"), "data2")

	reg := module.NewRegistry()
	registerTestModule(reg, "linux", "module1", risk.RiskSafe, []module.Artifact{
		{Path: path1, Method: "truncate", Risk: "safe"},
	})
	registerTestModule(reg, "linux", "module2", risk.RiskSafe, []module.Artifact{
		{Path: path2, Method: "truncate", Risk: "safe"},
	})

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	if err := eng.Run(RunOptions{ModuleNames: []string{"module1"}}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileEmpty(t, path1)
	assertFileContent(t, path2, "data2")
}

func TestEngineRunUnknownModule(t *testing.T) {
	homeDir := t.TempDir()
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)

	err := eng.Run(RunOptions{ModuleNames: []string{"nonexistent"}})
	if err == nil {
		t.Error("expected error for unknown module")
	}
}

func TestEngineRunWithBackup(t *testing.T) {
	homeDir := t.TempDir()
	backupDir := t.TempDir()
	content := "backed up data"
	logPath := writeFileAbs(t, filepath.Join(homeDir, "app.log"), content)

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: logPath, Method: "delete", Risk: "safe", Backup: true},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskSafe, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	if err := eng.Run(RunOptions{UseBackup: true, BackupDir: backupDir}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileGone(t, logPath)

	eng.backup.Restore()
	assertFileContent(t, logPath, content)
}

func TestEngineRunShred(t *testing.T) {
	homeDir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(homeDir, "secret.dat"), "classified information")

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: path, Method: "delete", Risk: "safe"},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskSafe, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	if err := eng.Run(RunOptions{UseShred: true}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileGone(t, path)
}

func TestEngineAllUserHomeExpansion(t *testing.T) {
	homeDir := t.TempDir()
	homeA := t.TempDir()
	homeB := t.TempDir()

	pathA := writeFileAbs(t, filepath.Join(homeA, ".bash_history"), "history a")
	pathB := writeFileAbs(t, filepath.Join(homeB, ".bash_history"), "history b")
	pathRoot := writeFileAbs(t, filepath.Join(homeDir, ".bash_history"), "history root")

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: "{{.HomeDir}}/.bash_history", Method: "truncate", Risk: "safe"},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskSafe, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	eng.homes = []string{homeA, homeB, homeDir}

	if err := eng.Run(RunOptions{}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileEmpty(t, pathA)
	assertFileEmpty(t, pathB)
	assertFileEmpty(t, pathRoot)
}

func TestEngineAllUserHomeExpansionDryRun(t *testing.T) {
	homeDir := t.TempDir()
	homeA := t.TempDir()

	content := "keep me"
	pathA := writeFileAbs(t, filepath.Join(homeA, ".bash_history"), content)

	reg := module.NewRegistry()
	arts := []module.Artifact{
		{Path: "{{.HomeDir}}/.bash_history", Method: "truncate", Risk: "safe"},
	}
	registerTestModule(reg, "linux", "testmod", risk.RiskSafe, arts)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	eng.homes = []string{homeA}

	if err := eng.Run(RunOptions{DryRun: true}); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertFileContent(t, pathA, content)
	if eng.GetStats().Cleaned < 1 {
		t.Errorf("dry-run should count expanded homes as cleaned, got %d", eng.GetStats().Cleaned)
	}
}

func TestEngineTimestompChangesMtime(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "history.txt"), "old content")

	before := statTime(t, path)

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.timestomp = true

	a := module.Artifact{Path: path, Method: "truncate", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileEmpty(t, path)

	after := statTime(t, path)
	if after.Equal(before) {
		t.Fatalf("timestomp should change mtime, before=%v after=%v", before, after)
	}

	diff := after.Sub(before)
	if diff < -8*24*time.Hour || diff > 8*24*time.Hour {
		t.Errorf("mtime change should be within ±7 days, got %v", diff)
	}
}

func TestEngineTimestompDryRunKeepsMtime(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "history.txt"), "keep me")

	before := statTime(t, path)

	eng, _ := newTestEngine(t)
	eng.dryRun = true
	eng.timestomp = true

	a := module.Artifact{Path: path, Method: "truncate", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileContent(t, path, "keep me")

	after := statTime(t, path)
	if !after.Equal(before) {
		t.Errorf("dry-run should not change mtime, before=%v after=%v", before, after)
	}
}

func TestEngineStripXattrAfterTruncate(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "history.txt"), "data")

	if err := setTestXattr(path, "user.lethe_test", "secret"); err != nil {
		t.Skipf("filesystem does not support xattr: %v", err)
	}

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.stripXattr = true

	a := module.Artifact{Path: path, Method: "truncate", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileEmpty(t, path)
	if hasTestXattr(t, path) {
		t.Error("xattr should be removed after clean with --strip-xattr")
	}
}

func TestEngineNoStripXattrByDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeFileAbs(t, filepath.Join(dir, "history.txt"), "data")

	if err := setTestXattr(path, "user.lethe_test", "secret"); err != nil {
		t.Skipf("filesystem does not support xattr: %v", err)
	}

	eng, _ := newTestEngine(t)
	eng.dryRun = false
	eng.stripXattr = false

	a := module.Artifact{Path: path, Method: "truncate", Risk: "safe"}
	eng.cleanArtifact("test", a)

	assertFileEmpty(t, path)
	if !hasTestXattr(t, path) {
		t.Error("xattr should remain when --strip-xattr is off")
	}
}

func TestEngineNoModulesAvailable(t *testing.T) {
	homeDir := t.TempDir()
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)

	err := eng.Run(RunOptions{})
	if err == nil {
		t.Error("expected error when no modules available")
	}
}

func TestResolvePathIntegration(t *testing.T) {
	homeDir := "/home/testuser"

	tests := []struct {
		input    string
		expected string
	}{
		{"{{.HomeDir}}/.bash_history", "/home/testuser/.bash_history"},
		{"/var/log/syslog", "/var/log/syslog"},
	}

	for _, tt := range tests {
		result := module.ResolvePath(tt.input, homeDir)
		if result != tt.expected {
			t.Errorf("ResolvePath(%q, %q) = %q, want %q", tt.input, homeDir, result, tt.expected)
		}
	}
}

func TestProcessDetectionNonexistent(t *testing.T) {
	if isProcessRunning("lethe-nonexistent-process-xyz") {
		t.Error("nonexistent process should not be detected as running")
	}
}

func TestProcessDetectionRunning(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skip("cannot start sleep process")
	}
	defer cmd.Process.Kill()

	if !isProcessRunning("sleep") {
		t.Error("sleep process should be detected as running")
	}

	cmd.Process.Kill()
	cmd.Wait()

	time.Sleep(100 * time.Millisecond)

	if isProcessRunning("sleep") {
		t.Log("sleep still detected (may be other sleep processes on system), not a failure")
	}
}
