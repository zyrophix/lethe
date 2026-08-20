package artifacts

import (
	"testing"

	"github.com/zyrophix/lethe/internal/module"
)

func TestLoadAllPlatforms(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			groups, err := Load(platform)
			if err != nil {
				t.Fatalf("load %s: %v", platform, err)
			}
			if len(groups) == 0 {
				t.Fatalf("no groups for %s", platform)
			}
		})
	}
}

func TestNoEmptyPaths(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			groups, err := Load(platform)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range groups {
				for i, a := range g.Artifacts {
					if a.Path == "" {
						t.Errorf("%s/%s artifact[%d]: empty path", platform, g.Module, i)
					}
				}
			}
		})
	}
}

func TestAllMethodsKnown(t *testing.T) {
	known := map[string]bool{
		"truncate": true, "delete": true, "shred": true,
		"wipe_registry": true, "sqlite": true, "system_command": true,
	}
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			groups, err := Load(platform)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range groups {
				for i, a := range g.Artifacts {
					if !known[a.Method] {
						t.Errorf("%s/%s artifact[%d]: unknown method %q", platform, g.Module, i, a.Method)
					}
				}
			}
		})
	}
}

func TestAllRiskLevelsValid(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			groups, err := Load(platform)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range groups {
				for i, a := range g.Artifacts {
					if _, err := module.ParseCleanMethod(a.Method); err != nil {
						t.Errorf("%s/%s artifact[%d]: invalid method %q", platform, g.Module, i, a.Method)
					}
					rl := a.GetRisk()
					if rl < 0 || rl > 2 {
						t.Errorf("%s/%s artifact[%d]: invalid risk level %q", platform, g.Module, i, a.Risk)
					}
				}
			}
		})
	}
}

func TestNoDuplicatePathsInModule(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			groups, err := Load(platform)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range groups {
				seen := make(map[string]bool)
				for _, a := range g.Artifacts {
					if seen[a.Path] {
						t.Errorf("%s/%s: duplicate path %q", platform, g.Module, a.Path)
					}
					seen[a.Path] = true
				}
			}
		})
	}
}

func TestDestructiveArtifactsHaveBackup(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			groups, err := Load(platform)
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range groups {
				for i, a := range g.Artifacts {
					if a.GetRisk() == 2 && !a.Backup {
						t.Errorf("%s/%s artifact[%d] (%s): destructive without backup=true", platform, g.Module, i, a.Path)
					}
				}
			}
		})
	}
}

func TestArtifactsForModule(t *testing.T) {
	arts, err := ArtifactsForModule("linux", "shell")
	if err != nil {
		t.Fatalf("shell module: %v", err)
	}
	if len(arts) == 0 {
		t.Fatal("shell module has no artifacts")
	}

	_, err = ArtifactsForModule("linux", "nonexistent")
	if err == nil {
		t.Error("nonexistent module should return error")
	}
}

func TestAllArtifactsForPlatform(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows"} {
		t.Run(platform, func(t *testing.T) {
			arts, err := AllArtifactsForPlatform(platform)
			if err != nil {
				t.Fatal(err)
			}
			if len(arts) == 0 {
				t.Fatalf("no artifacts for %s", platform)
			}
		})
	}
}
