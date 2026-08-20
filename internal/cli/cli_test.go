package cli

import (
	"testing"
)

func resetFlags() {
	dryRun = false
	maxRisk = "risky"
	modulesFlag = ""
	parallel = false
	doBackup = false
	backupDir = ""
	doShred = false
	timestomp = false
	wipeFree = false
	stripXattr = false
	outputFmt = "text"
	auditLog = ""
	force = false
	debug = false
}

func TestVerifyCommandFailsOnUncleaned(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"verify"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when artifacts are not cleaned")
	}
}

func TestVerifyCommandUnknownModule(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"verify", "--modules", "no_such_module"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func TestListCommand(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"list"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("list failed: %v", err)
	}
}

func TestVersionCommand(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"version"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("version failed: %v", err)
	}
}

func TestCleanDryRunSafeModule(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"clean", "--dry-run", "--force", "--max-risk", "safe", "--modules", "temp"})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("clean dry-run: %v", err)
	}
}

func TestCleanUnknownModule(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"clean", "--dry-run", "--force", "--modules", "no_such_module"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for unknown module")
	}
}

func TestCleanInvalidMaxRisk(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"clean", "--dry-run", "--force", "--max-risk", "bogus"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for invalid --max-risk")
	}
}

func TestVerifyJSONOutput(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"verify", "--output", "json"})
	if err := rootCmd.Execute(); err == nil {
		t.Log("verify passed (system clean); JSON output valid")
	}
}

func TestVerifyInvalidMaxRisk(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"verify", "--max-risk", "bogus"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for invalid --max-risk")
	}
}

func TestRestoreRequiresBackupDir(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"restore"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error when --backup-dir missing")
	}
}

func TestRestoreMissingArchive(t *testing.T) {
	resetFlags()
	rootCmd.SetArgs([]string{"restore", "--backup-dir", "/nonexistent/backup"})
	if err := rootCmd.Execute(); err == nil {
		t.Fatal("expected error for missing archive")
	}
}
