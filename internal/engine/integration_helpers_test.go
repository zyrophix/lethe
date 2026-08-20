//go:build integration

package engine

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/risk"
	"golang.org/x/sys/unix"
	_ "modernc.org/sqlite"
)

func setTestXattr(path, name, value string) error {
	return unix.Setxattr(path, name, []byte(value), 0)
}

func hasTestXattr(t *testing.T, path string) bool {
	t.Helper()
	size, err := unix.Llistxattr(path, nil)
	if err != nil {
		return false
	}
	buf := make([]byte, size)
	n, err := unix.Llistxattr(path, buf)
	if err != nil {
		return false
	}
	return n > 0
}

func statTime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.ModTime()
}

func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeFileAbs(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatalf("expected file to exist: %s", path)
	}
}

func assertFileGone(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be gone: %s", path)
	}
}

func assertFileEmpty(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected file to be empty, got size=%d: %s", info.Size(), path)
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != expected {
		t.Fatalf("content mismatch for %s: got %q, want %q", path, string(data), expected)
	}
}

func assertFileNotContent(t *testing.T, path, forbidden string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) == forbidden {
		t.Fatalf("file %s still contains forbidden content", path)
	}
}

func createTestArtifact(t *testing.T, dir string) string {
	t.Helper()
	marker := "LETHE-TEST-" + t.Name()
	return writeFile(t, dir, "artifact.txt", marker)
}

func newTestEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	homeDir := t.TempDir()
	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, homeDir)
	return eng, homeDir
}

func newTestEngineWithPolicy(t *testing.T, maxRisk risk.RiskLevel) (*Engine, string) {
	t.Helper()
	homeDir := t.TempDir()
	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(maxRisk), w, homeDir)
	return eng, homeDir
}

func registerTestModule(reg *module.Registry, platform, name string, riskLevel risk.RiskLevel, artifacts []module.Artifact) {
	arts := artifacts
	mod := module.NewYAMLModule(name, platform, riskLevel, nil, func() []module.Artifact {
		return arts
	})
	reg.Register(mod)
}

func createSQLiteDB(t *testing.T, path string, table string, rows int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	_, db := openSQLite(t, path)
	_, err := db.Exec(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (id INTEGER PRIMARY KEY, data TEXT)", quoteIdentifier(table)))
	if err != nil {
		t.Fatalf("create table: %v", err)
	}
	for i := 0; i < rows; i++ {
		_, err := db.Exec(fmt.Sprintf("INSERT INTO %s (data) VALUES (?)", quoteIdentifier(table)), "LETHE-TEST-ROW")
		if err != nil {
			t.Fatalf("insert row %d: %v", i, err)
		}
	}
	db.Close()
}

func openSQLite(t *testing.T, path string) (string, *sql.DB) {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open sqlite %s: %v", path, err)
	}
	return path, db
}

func countSQLiteRows(t *testing.T, path, table string) int {
	t.Helper()
	_, db := openSQLite(t, path+"?mode=ro")
	defer db.Close()
	var count int
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(table))).Scan(&count)
	if err != nil {
		t.Fatalf("count rows in %s: %v", table, err)
	}
	return count
}
