package artifacts

import (
	"embed"
	"fmt"

	"github.com/zyrophix/lethe/internal/module"
	"gopkg.in/yaml.v3"
)

//go:embed configs/artifacts/*.yaml
var configsFS embed.FS

type ArtifactGroup struct {
	Module    string            `yaml:"module"`
	Risk      string            `yaml:"risk"`
	Artifacts []module.Artifact `yaml:"artifacts"`
}

func Load(platform string) ([]ArtifactGroup, error) {
	path := fmt.Sprintf("configs/artifacts/%s.yaml", platform)
	data, err := configsFS.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no artifact config for platform %q: %w", platform, err)
	}

	var groups []ArtifactGroup
	if err := yaml.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("parse %s artifacts: %w", platform, err)
	}

	return groups, nil
}

func AllArtifactsForPlatform(platform string) ([]module.Artifact, error) {
	groups, err := Load(platform)
	if err != nil {
		return nil, err
	}

	var all []module.Artifact
	for _, g := range groups {
		all = append(all, g.Artifacts...)
	}
	return all, nil
}

func ArtifactsForModule(platform, moduleName string) ([]module.Artifact, error) {
	groups, err := Load(platform)
	if err != nil {
		return nil, err
	}

	for _, g := range groups {
		if g.Module == moduleName {
			return g.Artifacts, nil
		}
	}

	return nil, fmt.Errorf("module %q not found for platform %q", moduleName, platform)
}
