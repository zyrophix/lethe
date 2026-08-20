package engine

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lethe/lethe/internal/module"
	_ "modernc.org/sqlite"
)

type VerifyResult struct {
	Artifact module.Artifact
	Cleaned  bool
	Reason   string
}

func Verify(artifacts []module.Artifact, homeDir string) []VerifyResult {
	return verifyArtifacts(artifacts, []string{homeDir})
}

// VerifyAll verifies artifacts across multiple home directories, expanding
// {{.HomeDir}} paths over every home.
func VerifyAll(artifacts []module.Artifact, homes []string) []VerifyResult {
	return verifyArtifacts(artifacts, homes)
}

func verifyArtifacts(artifacts []module.Artifact, homes []string) []VerifyResult {
	var results []VerifyResult

	for _, a := range artifacts {
		result := VerifyResult{Artifact: a}

		switch a.GetMethod() {
		case module.MethodTruncate:
			result = verifyTruncate(a, resolveForVerify(a, homes))
		case module.MethodDelete, module.MethodShred:
			result = verifyDeleted(a, resolveForVerify(a, homes))
		case module.MethodSystemCommand:
			result.Cleaned = false
			result.Reason = "system command - manual verification required"
		case module.MethodSQLite:
			result = verifySQLite(a, resolveForVerify(a, homes))
		case module.MethodWipeRegistry:
			result = verifyRegistry(a, homes[0])
		default:
			result.Cleaned = false
			result.Reason = "unknown method - cannot verify"
		}

		results = append(results, result)
	}

	return results
}

func resolveForVerify(a module.Artifact, homes []string) []string {
	if !strings.Contains(a.Path, "{{.HomeDir}}") {
		if p := module.ResolvePath(a.Path, homes[0]); p != "" {
			return []string{p}
		}
		return nil
	}

	var paths []string
	for _, h := range homes {
		if p := module.ResolvePath(a.Path, h); p != "" {
			paths = append(paths, p)
		}
	}
	return paths
}

func verifyTruncate(a module.Artifact, paths []string) VerifyResult {
	var matched bool
	for _, resolved := range paths {
		matches, err := filepath.Glob(resolved)
		if err != nil {
			return VerifyResult{Artifact: a, Cleaned: false, Reason: "invalid path pattern"}
		}
		for _, m := range matches {
			matched = true
			info, err := os.Stat(m)
			if err != nil {
				continue
			}
			if info.Size() > 0 {
				return VerifyResult{Artifact: a, Cleaned: false, Reason: fmt.Sprintf("%s still has content (size=%d)", m, info.Size())}
			}
		}
	}

	if !matched {
		return VerifyResult{Artifact: a, Cleaned: true, Reason: "file does not exist (already clean)"}
	}
	return VerifyResult{Artifact: a, Cleaned: true, Reason: "file truncated to zero"}
}

func verifyDeleted(a module.Artifact, paths []string) VerifyResult {
	var existing []string
	for _, resolved := range paths {
		matches, err := filepath.Glob(resolved)
		if err != nil {
			return VerifyResult{Artifact: a, Cleaned: false, Reason: "invalid path pattern"}
		}
		existing = append(existing, matches...)
	}

	if len(existing) == 0 {
		return VerifyResult{Artifact: a, Cleaned: true, Reason: "path does not exist"}
	}

	return VerifyResult{Artifact: a, Cleaned: false, Reason: "path still exists: " + strings.Join(existing, ", ")}
}

func verifySQLite(a module.Artifact, paths []string) VerifyResult {
	table := a.SQLiteTable
	if table == "" {
		return VerifyResult{Artifact: a, Cleaned: false, Reason: "sqlite_table not specified"}
	}

	var matched bool
	for _, resolved := range paths {
		if _, err := os.Stat(resolved); os.IsNotExist(err) {
			continue
		}
		matched = true

		db, err := sql.Open("sqlite", resolved+"?mode=ro")
		if err != nil {
			return VerifyResult{Artifact: a, Cleaned: false, Reason: "cannot open sqlite database"}
		}

		query := fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteIdentifier(table))
		if a.SQLiteWhere != "" {
			query += " WHERE " + a.SQLiteWhere
		}

		var count int
		if err := db.QueryRow(query).Scan(&count); err != nil {
			db.Close()
			return VerifyResult{Artifact: a, Cleaned: false, Reason: fmt.Sprintf("query failed: %v", err)}
		}
		db.Close()

		if count > 0 {
			return VerifyResult{Artifact: a, Cleaned: false, Reason: fmt.Sprintf("%d rows remaining in %s", count, table)}
		}
	}

	if !matched {
		return VerifyResult{Artifact: a, Cleaned: true, Reason: "database does not exist (already clean)"}
	}
	return VerifyResult{Artifact: a, Cleaned: true, Reason: "table is empty"}
}
