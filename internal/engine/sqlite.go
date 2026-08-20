package engine

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/lethe/lethe/internal/module"
	_ "modernc.org/sqlite"
)

func cleanSQLite(a module.Artifact, resolved string, dryRun bool) error {
	if _, err := os.Stat(resolved); os.IsNotExist(err) {
		return nil
	}

	db, err := sql.Open("sqlite", resolved+"?mode=rw")
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	table := a.SQLiteTable
	if table == "" {
		return fmt.Errorf("sqlite_table not specified for %s", a.Path)
	}

	if dryRun {
		var count int
		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(table))
		if a.SQLiteWhere != "" {
			query += " WHERE " + a.SQLiteWhere
		}
		if err := db.QueryRow(query).Scan(&count); err != nil {
			return fmt.Errorf("count rows: %w", err)
		}
		return nil
	}

	deleteQuery := fmt.Sprintf("DELETE FROM %s", quoteIdentifier(table))
	if a.SQLiteWhere != "" {
		deleteQuery += " WHERE " + a.SQLiteWhere
	}

	if _, err := db.Exec(deleteQuery); err != nil {
		return fmt.Errorf("delete from %s: %w", table, err)
	}

	if _, err := db.Exec("VACUUM"); err != nil {
		return fmt.Errorf("vacuum: %w", err)
	}

	return nil
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
