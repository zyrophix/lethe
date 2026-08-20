package module

import (
	"testing"

	"github.com/zyrophix/lethe/internal/risk"
)

type testModule struct {
	name      string
	risk      risk.RiskLevel
	platforms []string
}

func (m testModule) Name() string                  { return m.name }
func (m testModule) Risk() risk.RiskLevel          { return m.risk }
func (m testModule) Platforms() []string           { return m.platforms }
func (m testModule) Artifacts() []Artifact         { return nil }
func (m testModule) CustomClean(ctx Context) error { return nil }

func TestRegistryRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	m := testModule{name: "shell", risk: risk.RiskSafe, platforms: []string{"linux", "darwin"}}
	r.Register(m)

	got, ok := r.GetForPlatform("linux", "shell")
	if !ok {
		t.Fatal("expected module to be found for linux")
	}
	if got.Name() != "shell" {
		t.Errorf("got name %q, want %q", got.Name(), "shell")
	}

	_, ok = r.GetForPlatform("linux", "nonexistent")
	if ok {
		t.Error("expected nonexistent module to not be found")
	}
}

func TestRegistryListForPlatform(t *testing.T) {
	r := NewRegistry()
	r.Register(testModule{name: "zebra", risk: risk.RiskSafe, platforms: []string{"linux"}})
	r.Register(testModule{name: "alpha", risk: risk.RiskSafe, platforms: []string{"linux"}})

	list := r.ListForPlatform("linux")
	if len(list) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(list))
	}
	if list[0].Name() != "alpha" || list[1].Name() != "zebra" {
		t.Errorf("modules not sorted: got %q, %q", list[0].Name(), list[1].Name())
	}
}

func TestRegistryListForPlatformMulti(t *testing.T) {
	r := NewRegistry()
	r.Register(testModule{name: "shell", risk: risk.RiskSafe, platforms: []string{"linux", "darwin"}})
	r.Register(testModule{name: "macos", risk: risk.RiskSafe, platforms: []string{"darwin"}})
	r.Register(testModule{name: "events", risk: risk.RiskSafe, platforms: []string{"windows"}})

	linuxMods := r.ListForPlatform("linux")
	if len(linuxMods) != 1 {
		t.Fatalf("expected 1 linux module, got %d", len(linuxMods))
	}
	if linuxMods[0].Name() != "shell" {
		t.Errorf("expected shell, got %q", linuxMods[0].Name())
	}

	darwinMods := r.ListForPlatform("darwin")
	if len(darwinMods) != 2 {
		t.Fatalf("expected 2 darwin modules, got %d", len(darwinMods))
	}

	winMods := r.ListForPlatform("windows")
	if len(winMods) != 1 {
		t.Fatalf("expected 1 windows module, got %d", len(winMods))
	}
}

func TestArtifactGetRisk(t *testing.T) {
	a := Artifact{Risk: "destructive"}
	if a.GetRisk() != risk.RiskDestructive {
		t.Errorf("expected destructive, got %d", a.GetRisk())
	}

	a2 := Artifact{Risk: "invalid"}
	if a2.GetRisk() != risk.RiskSafe {
		t.Errorf("expected safe fallback for invalid risk, got %d", a2.GetRisk())
	}
}

func TestArtifactGetMethod(t *testing.T) {
	a := Artifact{Method: "truncate"}
	if a.GetMethod() != MethodTruncate {
		t.Errorf("expected truncate, got %d", a.GetMethod())
	}

	a2 := Artifact{Method: "invalid"}
	if a2.GetMethod() != MethodDelete {
		t.Errorf("expected delete fallback for invalid method, got %d", a2.GetMethod())
	}
}
