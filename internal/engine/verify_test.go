package engine

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/zyrophix/lethe/internal/module"
	_ "modernc.org/sqlite"
)

func createSQLiteTestDB(t *testing.T, path, table string, rows int) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()

	if _, err := db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY, data TEXT)", quoteIdentifier(table))); err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 0; i < rows; i++ {
		if _, err := db.Exec(fmt.Sprintf("INSERT INTO %s (data) VALUES (?)", quoteIdentifier(table)), "data"); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
}

func TestVerifyTruncateEmpty(t *testing.T) {
	path := tmpFile(t, "")
	defer os.Remove(path)

	artifacts := []module.Artifact{
		{Path: path, Method: "truncate", Risk: "safe"},
	}
	results := Verify(artifacts, "")
	if len(results) != 1 {
		t.Fatalf("results: got %d, want 1", len(results))
	}
	if !results[0].Cleaned {
		t.Errorf("empty file should be verified as cleaned: %s", results[0].Reason)
	}
}

func TestVerifyTruncateWithContent(t *testing.T) {
	path := tmpFile(t, "sensitive data")
	defer os.Remove(path)

	artifacts := []module.Artifact{
		{Path: path, Method: "truncate", Risk: "safe"},
	}
	results := Verify(artifacts, "")
	if results[0].Cleaned {
		t.Error("file with content should NOT be verified as cleaned")
	}
}

func TestVerifyDeletedFileGone(t *testing.T) {
	artifacts := []module.Artifact{
		{Path: "/tmp/lethe-nonexistent-verify-test", Method: "delete", Risk: "safe"},
	}
	results := Verify(artifacts, "")
	if !results[0].Cleaned {
		t.Errorf("nonexistent file should be verified as cleaned: %s", results[0].Reason)
	}
}

func TestVerifyDeletedFileExists(t *testing.T) {
	path := tmpFile(t, "data")
	defer os.Remove(path)

	artifacts := []module.Artifact{
		{Path: path, Method: "delete", Risk: "safe"},
	}
	results := Verify(artifacts, "")
	if results[0].Cleaned {
		t.Error("existing file should NOT be verified as cleaned for delete method")
	}
}

func TestVerifySQLite(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	createSQLiteTestDB(t, dbPath, "history", 0)
	artifacts := []module.Artifact{
		{Path: dbPath, Method: "sqlite", Risk: "risky", SQLiteTable: "history"},
	}
	results := Verify(artifacts, "")
	if !results[0].Cleaned {
		t.Errorf("empty sqlite table should verify as cleaned: %s", results[0].Reason)
	}
}

func TestVerifySQLiteWithRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	createSQLiteTestDB(t, dbPath, "history", 10)
	artifacts := []module.Artifact{
		{Path: dbPath, Method: "sqlite", Risk: "risky", SQLiteTable: "history"},
	}
	results := Verify(artifacts, "")
	if results[0].Cleaned {
		t.Errorf("sqlite table with rows should NOT verify as cleaned: %s", results[0].Reason)
	}
}

func TestVerifySQLiteNonexistentDB(t *testing.T) {
	artifacts := []module.Artifact{
		{Path: "/tmp/lethe-nonexistent-verify.db", Method: "sqlite", Risk: "risky", SQLiteTable: "history"},
	}
	results := Verify(artifacts, "")
	if !results[0].Cleaned {
		t.Errorf("nonexistent sqlite db should verify as cleaned: %s", results[0].Reason)
	}
}

func TestVerifyWipeRegistry(t *testing.T) {
	artifacts := []module.Artifact{
		{Path: "HKLM\\Software\\Test", Method: "wipe_registry", Risk: "destructive"},
	}
	results := Verify(artifacts, "")
	if results[0].Cleaned {
		t.Error("wipe_registry should not be verified as cleaned (not implemented)")
	}
}

func TestVerifySystemCommand(t *testing.T) {
	artifacts := []module.Artifact{
		{Path: "", Method: "system_command", Risk: "safe"},
	}
	results := Verify(artifacts, "")
	if results[0].Cleaned {
		t.Error("system_command cannot be auto-verified")
	}
	if results[0].Reason != "system command - manual verification required" {
		t.Errorf("unexpected reason: %q", results[0].Reason)
	}
}

func TestVerifyTruncateNoFile(t *testing.T) {
	artifacts := []module.Artifact{
		{Path: "/tmp/lethe-no-such-file", Method: "truncate", Risk: "safe"},
	}
	results := Verify(artifacts, "")
	if !results[0].Cleaned {
		t.Errorf("nonexistent file for truncate should be cleaned: %s", results[0].Reason)
	}
}

func TestVerifyAllExpandsHomes(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()

	writeVerifyFile(t, filepath.Join(homeA, ".bash_history"), "history a")
	writeVerifyFile(t, filepath.Join(homeB, ".bash_history"), "history b")

	artifacts := []module.Artifact{
		{Path: "{{.HomeDir}}/.bash_history", Method: "truncate", Risk: "safe"},
	}
	results := VerifyAll(artifacts, []string{homeA, homeB})
	if len(results) != 1 {
		t.Fatalf("results: got %d, want 1", len(results))
	}
	if results[0].Cleaned {
		t.Errorf("files with content should NOT verify as cleaned: %s", results[0].Reason)
	}

	os.Truncate(filepath.Join(homeA, ".bash_history"), 0)
	os.Truncate(filepath.Join(homeB, ".bash_history"), 0)

	results = VerifyAll(artifacts, []string{homeA, homeB})
	if !results[0].Cleaned {
		t.Errorf("truncated files should verify as cleaned: %s", results[0].Reason)
	}
}

func TestVerifyAllMissingOneHome(t *testing.T) {
	homeA := t.TempDir()
	homeB := t.TempDir()

	writeVerifyFile(t, filepath.Join(homeA, ".bash_history"), "history a")

	artifacts := []module.Artifact{
		{Path: "{{.HomeDir}}/.bash_history", Method: "truncate", Risk: "safe"},
	}
	results := VerifyAll(artifacts, []string{homeA, homeB})
	if results[0].Cleaned {
		t.Errorf("one home still has content, should NOT be cleaned: %s", results[0].Reason)
	}
}

func writeVerifyFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}
