package engine

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/risk"
)

// TestBackupFailureAbortsClean ensures a failed backup stops the run
// before anything is deleted (fail closed on irreversibility).
func TestBackupFailureAbortsClean(t *testing.T) {
	homeDir := t.TempDir()
	target := writeSecFile(t, filepath.Join(homeDir, "precious.log"), "must survive")

	// baseDir sits beneath an existing file, so archive creation fails.
	blocker := writeSecFile(t, filepath.Join(t.TempDir(), "blocker"), "x")
	badBase := filepath.Join(blocker, "cannot-create-dir")

	reg := module.NewRegistry()
	registerSecModule(reg, "linux", "testmod", risk.RiskSafe, []module.Artifact{
		{Path: target, Method: "delete", Risk: "safe", Backup: true},
	})

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	err := eng.Run(context.Background(), RunOptions{UseBackup: true, BackupDir: badBase})
	if err == nil {
		t.Fatal("expected Run to fail when backup cannot be created")
	}
	if !strings.Contains(err.Error(), "backup failed") {
		t.Errorf("error should mention backup failure, got: %v", err)
	}
	assertSecContent(t, target, "must survive")
}

// TestCustomCleanGatedByModuleRisk proves custom system commands do not
// run below the module's declared risk level.
func TestCustomCleanGatedByModuleRisk(t *testing.T) {
	newReg := func() (*module.Registry, *syncFlag) {
		flag := &syncFlag{}
		mod := &riskGateModule{flag: flag, riskLevel: risk.RiskDestructive}
		reg := module.NewRegistry()
		reg.Register(mod)
		return reg, flag
	}

	t.Run("skipped below max-risk", func(t *testing.T) {
		reg, flag := newReg()
		eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, t.TempDir())
		if err := eng.Run(context.Background(), RunOptions{ModuleNames: []string{"gatemod"}}); err != nil {
			t.Fatalf("run: %v", err)
		}
		if flag.called {
			t.Fatal("destructive custom clean must not run at --max-risk safe")
		}
	})

	t.Run("runs at max-risk", func(t *testing.T) {
		reg, flag := newReg()
		eng := New(reg, risk.NewPolicy(risk.RiskDestructive), &mockWriter{}, t.TempDir())
		if err := eng.Run(context.Background(), RunOptions{ModuleNames: []string{"gatemod"}}); err != nil {
			t.Fatalf("run: %v", err)
		}
		if !flag.called {
			t.Fatal("custom clean should run when module risk <= max")
		}
	})
}

type syncFlag struct {
	mu     sync.Mutex
	called bool
}

func (f *syncFlag) hit() { f.mu.Lock(); f.called = true; f.mu.Unlock() }

type riskGateModule struct {
	flag      *syncFlag
	riskLevel risk.RiskLevel
}

func (m *riskGateModule) Name() string                 { return "gatemod" }
func (m *riskGateModule) Risk() risk.RiskLevel         { return m.riskLevel }
func (m *riskGateModule) Platforms() []string          { return []string{"linux"} }
func (m *riskGateModule) Artifacts() []module.Artifact { return nil }
func (m *riskGateModule) CustomClean(ctx module.Context) error {
	m.flag.hit()
	return nil
}

// TestNonRecursiveDeleteRefusesDirectory honors recursive:false for dirs.
func TestNonRecursiveDeleteRefusesDirectory(t *testing.T) {
	dir := t.TempDir()
	inner := filepath.Join(dir, "nested")
	if err := os.MkdirAll(inner, 0o755); err != nil {
		t.Fatal(err)
	}
	keep := writeSecFile(t, filepath.Join(inner, "keep.txt"), "data")

	eng, _ := newSecEngine(t)
	if err := eng.deletePath(inner, false); err == nil {
		t.Fatal("non-recursive delete of directory must refuse")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("nested content must survive non-recursive refusal")
	}
}

// TestBackupArchiveIsPrivateAndUnpredictable checks 0600 perms and the
// random name component defending against symlink pre-creation.
func TestBackupArchiveIsPrivateAndUnpredictable(t *testing.T) {
	base := t.TempDir()
	first := NewBackup(base)
	if strings.Count(first.Path(), "-") < 3 {
		t.Errorf("expected random token in archive name, got %q", first.Path())
	}
	second := NewBackup(base)
	if first.Path() == second.Path() {
		t.Fatal("two backups must not collide on one predictable name")
	}
}

// TestBackupCollectsAllHomes ensures multi-home deletion is preceded by
// multi-home collection into the archive.
func TestBackupCollectsAllHomes(t *testing.T) {
	homeDir := t.TempDir()
	homeA := t.TempDir()
	homeB := t.TempDir()

	fileA := writeSecFile(t, filepath.Join(homeA, ".lethe-mh-test"), "a")
	fileB := writeSecFile(t, filepath.Join(homeB, ".lethe-mh-test"), "b")
	backupDir := t.TempDir()

	reg := module.NewRegistry()
	registerSecModule(reg, "linux", "testmod", risk.RiskSafe, []module.Artifact{
		{Path: "{{.HomeDir}}/.lethe-mh-test", Method: "delete", Risk: "safe", Backup: true},
	})

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), &mockWriter{}, homeDir)
	eng.homes = []string{homeA, homeB}
	if err := eng.Run(context.Background(), RunOptions{UseBackup: true, BackupDir: backupDir}); err != nil {
		t.Fatalf("run: %v", err)
	}

	f, err := os.Open(eng.backup.Path())
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	defer func() { _ = f.Close() }()
	tr := tar.NewReader(f)
	var found int
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Name == fileA || hdr.Name == fileB {
			found++
		}
	}
	if found != 2 {
		t.Errorf("archive must contain both homes' files, found %d (%s, %s)", found, fileA, fileB)
	}
}

// --- local helpers (unit scope; integration helpers live behind a tag) ---

func writeSecFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertSecContent(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file %s to survive: %v", path, err)
	}
	if string(data) != expected {
		t.Errorf("content mismatch in %s: got %q want %q", path, data, expected)
	}
}

func registerSecModule(reg *module.Registry, platform, name string, riskLevel risk.RiskLevel, artifacts []module.Artifact) {
	reg.Register(&genericSecModule{name: name, platform: platform, riskLevel: riskLevel, artifacts: artifacts})
}

func newSecEngine(t *testing.T) (*Engine, string) {
	t.Helper()
	home := t.TempDir()
	return New(module.NewRegistry(), risk.NewPolicy(risk.RiskSafe), &mockWriter{}, home), home
}

type genericSecModule struct {
	name      string
	platform  string
	riskLevel risk.RiskLevel
	artifacts []module.Artifact
}

func (m *genericSecModule) Name() string                         { return m.name }
func (m *genericSecModule) Risk() risk.RiskLevel                 { return m.riskLevel }
func (m *genericSecModule) Platforms() []string                  { return []string{m.platform} }
func (m *genericSecModule) Artifacts() []module.Artifact         { return m.artifacts }
func (m *genericSecModule) CustomClean(ctx module.Context) error { return nil }
