package windows

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	"github.com/zyrophix/lethe/internal/artifacts"
	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/risk"
)

func eventsClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	out, err := exec.Command("wevtutil", "el").Output()
	if err != nil {
		return fmt.Errorf("list event logs: %w", err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		logName := strings.TrimSpace(scanner.Text())
		if logName == "" {
			continue
		}
		if err := exec.Command("wevtutil", "cl", logName).Run(); err != nil {
			continue
		}
	}
	return nil
}

func tempClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := exec.Command("ipconfig", "/flushdns").Run(); err != nil {
		return fmt.Errorf("flushdns: %w", err)
	}
	return nil
}

func RegisterAll(registry *module.Registry) {
	customCleans := map[string]func(module.Context) error{
		"events":   eventsClean,
		"temp":     tempClean,
		"journal":  usnClean,
		"pagefile": pagefileClean,
		"shadows":  shadowsClean,
	}

	modules := []struct {
		name string
		risk risk.RiskLevel
	}{
		{"events", risk.RiskRisky},
		{"history", risk.RiskRisky},
		{"registry", risk.RiskDestructive},
		{"filesystem", risk.RiskRisky},
		{"temp", risk.RiskRisky},
		{"security", risk.RiskDestructive},
		{"advanced", risk.RiskDestructive},
		{"journal", risk.RiskDestructive},
		{"pagefile", risk.RiskDestructive},
		{"shadows", risk.RiskDestructive},
	}

	for _, m := range modules {
		name := m.name
		artFn := func() []module.Artifact {
			arts, err := artifacts.ArtifactsForModule("windows", name)
			if err != nil {
				return nil
			}
			return arts
		}
		registry.Register(module.NewYAMLModule(m.name, "windows", m.risk, customCleans[m.name], artFn))
	}
}
