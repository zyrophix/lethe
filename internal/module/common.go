package module

import (
	"github.com/zyrophix/lethe/internal/risk"
)

type BaseModule struct {
	NameField      string
	RiskLevelField risk.RiskLevel
	PlatformField  string
	CustomCleanFn  func(Context) error
	ArtifactsFn    func() []Artifact
}

func (m *BaseModule) Name() string         { return m.NameField }
func (m *BaseModule) Risk() risk.RiskLevel { return m.RiskLevelField }
func (m *BaseModule) Platforms() []string  { return []string{m.PlatformField} }

func (m *BaseModule) Artifacts() []Artifact {
	if m.ArtifactsFn != nil {
		return m.ArtifactsFn()
	}
	return nil
}

func (m *BaseModule) CustomClean(ctx Context) error {
	if m.CustomCleanFn != nil {
		return m.CustomCleanFn(ctx)
	}
	return nil
}

type YAMLModule struct {
	*BaseModule
	customClean func(Context) error
}

func NewYAMLModule(name, platform string, riskLevel risk.RiskLevel, customClean func(Context) error, artifactsFn func() []Artifact) *YAMLModule {
	return &YAMLModule{
		BaseModule: &BaseModule{
			NameField:      name,
			RiskLevelField: riskLevel,
			PlatformField:  platform,
			ArtifactsFn:    artifactsFn,
		},
		customClean: customClean,
	}
}

func (m *YAMLModule) CustomClean(ctx Context) error {
	if m.customClean != nil {
		return m.customClean(ctx)
	}
	return nil
}
