//go:build integration

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zyrophix/lethe/internal/module"
)

func TestSQLiteDeleteRows(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	createSQLiteDB(t, dbPath, "history", 100)

	if count := countSQLiteRows(t, dbPath, "history"); count != 100 {
		t.Fatalf("expected 100 rows before clean, got %d", count)
	}

	a := module.Artifact{
		Path:        dbPath,
		Method:      "sqlite",
		Risk:        "risky",
		SQLiteTable: "history",
	}

	if err := cleanSQLite(a, dbPath, false); err != nil {
		t.Fatalf("cleanSQLite: %v", err)
	}

	if count := countSQLiteRows(t, dbPath, "history"); count != 0 {
		t.Errorf("expected 0 rows after clean, got %d", count)
	}
}

func TestSQLiteDeleteWithWhere(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	createSQLiteDB(t, dbPath, "entries", 50)

	_, db := openSQLite(t, dbPath)
	db.Exec("INSERT INTO entries (data) VALUES ('KEEP-THIS')")
	db.Close()

	if count := countSQLiteRows(t, dbPath, "entries"); count != 51 {
		t.Fatalf("expected 51 rows, got %d", count)
	}

	a := module.Artifact{
		Path:        dbPath,
		Method:      "sqlite",
		Risk:        "risky",
		SQLiteTable: "entries",
		SQLiteWhere: "data = 'LETHE-TEST-ROW'",
	}

	if err := cleanSQLite(a, dbPath, false); err != nil {
		t.Fatalf("cleanSQLite with WHERE: %v", err)
	}

	if count := countSQLiteRows(t, dbPath, "entries"); count != 1 {
		t.Errorf("expected 1 row (KEEP-THIS) after selective delete, got %d", count)
	}
}

func TestSQLiteVacuumReducesSize(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	createSQLiteDB(t, dbPath, "bigtable", 1000)

	info, _ := os.Stat(dbPath)
	sizeBefore := info.Size()

	a := module.Artifact{
		Path:        dbPath,
		Method:      "sqlite",
		Risk:        "risky",
		SQLiteTable: "bigtable",
	}

	if err := cleanSQLite(a, dbPath, false); err != nil {
		t.Fatalf("cleanSQLite: %v", err)
	}

	info, _ = os.Stat(dbPath)
	sizeAfter := info.Size()

	if sizeAfter >= sizeBefore {
		t.Errorf("expected size to decrease after VACUUM: before=%d after=%d", sizeBefore, sizeAfter)
	}
}

func TestSQLiteDryRun(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	createSQLiteDB(t, dbPath, "history", 50)

	a := module.Artifact{
		Path:        dbPath,
		Method:      "sqlite",
		Risk:        "risky",
		SQLiteTable: "history",
	}

	if err := cleanSQLite(a, dbPath, true); err != nil {
		t.Fatalf("cleanSQLite dry-run: %v", err)
	}

	if count := countSQLiteRows(t, dbPath, "history"); count != 50 {
		t.Errorf("dry-run should NOT delete rows, got %d (want 50)", count)
	}
}

func TestSQLiteNonexistentDB(t *testing.T) {
	a := module.Artifact{
		Path:        "/tmp/lethe-nonexistent-db-test.db",
		Method:      "sqlite",
		Risk:        "risky",
		SQLiteTable: "history",
	}

	if err := cleanSQLite(a, "/tmp/lethe-nonexistent-db-test.db", false); err != nil {
		t.Errorf("nonexistent db should not error, got: %v", err)
	}
}

func TestSQLiteMissingTable(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	createSQLiteDB(t, dbPath, "sometable", 10)

	a := module.Artifact{
		Path:   dbPath,
		Method: "sqlite",
		Risk:   "risky",
	}

	if err := cleanSQLite(a, dbPath, false); err == nil {
		t.Error("expected error when sqlite_table not specified")
	}
}
