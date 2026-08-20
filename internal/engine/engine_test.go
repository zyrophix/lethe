package engine

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/risk"
)

type mockWriter struct {
	infos     []string
	warnings  []string
	errors    []string
	successes []string
	debugs    []string
}

func (w *mockWriter) Info(mod, msg string) { w.infos = append(w.infos, mod+":"+msg) }
func (w *mockWriter) Success(mod, art, act string, r risk.RiskLevel) {
	w.successes = append(w.successes, mod+":"+art)
}
func (w *mockWriter) Warning(msg string)                                { w.warnings = append(w.warnings, msg) }
func (w *mockWriter) Error(msg string)                                  { w.errors = append(w.errors, msg) }
func (w *mockWriter) Debug(mod, msg string)                             { w.debugs = append(w.debugs, mod+":"+msg) }
func (w *mockWriter) Summary(c, f, s, b int, d time.Duration, dry bool) {}
func (w *mockWriter) Flush()                                            {}

type mockModule struct {
	name      string
	riskLevel risk.RiskLevel
	platforms []string
	artifacts []module.Artifact
	cleanErr  error
}

func (m *mockModule) Name() string                         { return m.name }
func (m *mockModule) Risk() risk.RiskLevel                 { return m.riskLevel }
func (m *mockModule) Platforms() []string                  { return m.platforms }
func (m *mockModule) Artifacts() []module.Artifact         { return m.artifacts }
func (m *mockModule) CustomClean(ctx module.Context) error { return m.cleanErr }

func tmpFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "lethe-test-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	f.Close()
	return name
}

func tmpDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("", "lethe-testdir-*")
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestStatsConcurrent(t *testing.T) {
	s := &Stats{}
	var wg sync.WaitGroup
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.IncCleaned()
			s.IncFailed()
			s.IncSkipped()
			s.IncBackedUp()
		}()
	}
	wg.Wait()
	if s.Cleaned != 1000 {
		t.Errorf("Cleaned: got %d, want 1000", s.Cleaned)
	}
	if s.Failed != 1000 {
		t.Errorf("Failed: got %d, want 1000", s.Failed)
	}
	if s.Skipped != 1000 {
		t.Errorf("Skipped: got %d, want 1000", s.Skipped)
	}
	if s.BackedUp != 1000 {
		t.Errorf("BackedUp: got %d, want 1000", s.BackedUp)
	}
}

func TestCleanArtifactTruncate(t *testing.T) {
	path := tmpFile(t, "sensitive data")
	defer os.Remove(path)

	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "truncate", Risk: "safe"}
	eng.cleanArtifact("test", a)

	if _, err := os.Stat(path); err != nil {
		t.Fatal("file should still exist after truncate")
	}
	data, _ := os.ReadFile(path)
	if string(data) != "" {
		t.Errorf("file should be empty after truncate, got %q", data)
	}
	if eng.stats.Cleaned != 1 {
		t.Errorf("Cleaned: got %d, want 1", eng.stats.Cleaned)
	}
}

func TestCleanArtifactDelete(t *testing.T) {
	path := tmpFile(t, "data")

	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")
	eng.dryRun = false

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be deleted")
	}
	if eng.stats.Cleaned != 1 {
		t.Errorf("Cleaned: got %d, want 1", eng.stats.Cleaned)
	}
}

func TestCleanArtifactDeleteRecursive(t *testing.T) {
	dir := tmpDir(t)
	sub := filepath.Join(dir, "subdir")
	os.MkdirAll(sub, 0755)
	os.WriteFile(filepath.Join(sub, "file.txt"), []byte("data"), 0644)
	defer os.RemoveAll(dir)

	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")
	eng.dryRun = false

	a := module.Artifact{Path: dir, Method: "delete", Risk: "safe", Recursive: true}
	eng.cleanArtifact("test", a)

	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("directory should be deleted")
	}
}

func TestCleanArtifactDryRun(t *testing.T) {
	path := tmpFile(t, "data")
	defer os.Remove(path)

	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")
	eng.dryRun = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file should NOT be deleted in dry-run")
	}
	if eng.stats.Cleaned != 1 {
		t.Errorf("Cleaned: got %d, want 1 (dry-run counts as cleaned)", eng.stats.Cleaned)
	}
}

func TestCleanArtifactShred(t *testing.T) {
	path := tmpFile(t, "sensitive data that needs shredding")
	defer os.Remove(path)

	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")
	eng.dryRun = false
	eng.useShred = true

	a := module.Artifact{Path: path, Method: "delete", Risk: "safe"}
	eng.cleanArtifact("test", a)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should be deleted after shred")
	}
}

func TestCleanArtifactExclude(t *testing.T) {
	dir := tmpDir(t)
	defer os.RemoveAll(dir)
	keep := filepath.Join(dir, "important.log")
	junk := filepath.Join(dir, "junk.log")
	os.WriteFile(keep, []byte("keep"), 0644)
	os.WriteFile(junk, []byte("junk"), 0644)

	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")
	eng.dryRun = false

	a := module.Artifact{Path: dir + "/*.log", Method: "delete", Risk: "safe", Exclude: []string{"important.log"}}
	eng.cleanArtifact("test", a)

	if _, err := os.Stat(keep); os.IsNotExist(err) {
		t.Error("important.log should be excluded from deletion")
	}
	if _, err := os.Stat(junk); !os.IsNotExist(err) {
		t.Error("junk.log should be deleted")
	}
}

func TestRunRiskPolicyFiltering(t *testing.T) {
	safePath := tmpFile(t, "safe data")
	riskyPath := tmpFile(t, "risky data")
	destructivePath := tmpFile(t, "destructive data")
	defer os.Remove(safePath)
	defer os.Remove(riskyPath)
	defer os.Remove(destructivePath)

	w := &mockWriter{}
	reg := module.NewRegistry()

	mod := &mockModule{
		name:      "testmod",
		riskLevel: risk.RiskSafe,
		platforms: []string{"linux"},
		artifacts: []module.Artifact{
			{Path: safePath, Method: "truncate", Risk: "safe"},
			{Path: riskyPath, Method: "truncate", Risk: "risky"},
			{Path: destructivePath, Method: "delete", Risk: "destructive"},
		},
	}
	reg.Register(mod)

	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")
	err := eng.Run(RunOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}

	stats := eng.GetStats()
	if stats.Cleaned != 1 {
		t.Errorf("only safe should be cleaned: got %d", stats.Cleaned)
	}
	if stats.Skipped < 2 {
		t.Errorf("risky+destructive should be skipped: got %d skipped", stats.Skipped)
	}
}

func TestRunUnknownModule(t *testing.T) {
	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")

	err := eng.Run(RunOptions{ModuleNames: []string{"nonexistent"}})
	if err == nil {
		t.Error("expected error for unknown module")
	}
}

func TestShouldExclude(t *testing.T) {
	w := &mockWriter{}
	reg := module.NewRegistry()
	eng := New(reg, risk.NewPolicy(risk.RiskSafe), w, "/tmp")

	if !eng.shouldExclude("/tmp/important.log", []string{"important.log"}) {
		t.Error("should exclude important.log")
	}
	if eng.shouldExclude("/tmp/other.log", []string{"important.log"}) {
		t.Error("should NOT exclude other.log")
	}
}
