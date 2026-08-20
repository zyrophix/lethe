package darwin

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/lethe/lethe/internal/artifacts"
	"github.com/lethe/lethe/internal/module"
	"github.com/lethe/lethe/internal/risk"
)

// execCommand is injectable for tests.
var execCommand = func(name string, args ...string) *exec.Cmd {
	return exec.Command(name, args...)
}

var checkRoot = func() error {
	if os.Getuid() != 0 {
		return fmt.Errorf("this operation requires root (run with sudo)")
	}
	return nil
}

func macosClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := checkRoot(); err != nil {
		return err
	}
	if err := execCommand("find", "/", "-name", ".DS_Store", "-delete").Run(); err != nil {
		return fmt.Errorf("find .DS_Store: %w", err)
	}
	if err := execCommand("find", "/Users/*/.Trash", "-mindepth", "1", "-delete").Run(); err != nil {
		return fmt.Errorf("clear all users trash: %w", err)
	}
	if err := execCommand("mdutil", "-E", "/").Run(); err != nil {
		return fmt.Errorf("mdutil -E: %w", err)
	}
	if err := execCommand("qlmanage", "-r", "cache").Run(); err != nil {
		return fmt.Errorf("qlmanage reset: %w", err)
	}
	return nil
}

func auditDarwinClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := checkRoot(); err != nil {
		return err
	}
	if err := execCommand("audit", "-t").Run(); err != nil {
		return fmt.Errorf("audit -t: %w", err)
	}
	if err := execCommand("audit", "-s").Run(); err != nil {
		return fmt.Errorf("audit -s: %w", err)
	}
	return nil
}

func unifiedClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := checkRoot(); err != nil {
		return err
	}
	if err := gateUnifiedLogs(); err != nil {
		return err
	}
	return execCommand("log", "erase", "--all").Run()
}

func usageClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	for _, key := range []string{"RecentDocuments", "RecentApplications", "RecentServers"} {
		if err := execCommand("defaults", "delete", "com.apple.recentitems", key).Run(); err != nil {
			return fmt.Errorf("defaults delete %s: %w", key, err)
		}
	}
	return nil
}

// fseventsClean removes FSEvents logs at all levels, not just the root one.
func fseventsClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := checkRoot(); err != nil {
		return err
	}
	return execCommand("find", "/", "-name", ".fseventsd", "-type", "d", "-exec", "rm", "-rf", "{}", "+").Run()
}

// macosVersion returns the numeric macOS version (e.g. 10.15.7, 13.2) or "".
var macosVersion = func() (string, error) {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return "", fmt.Errorf("sw_vers: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// gateUnifiedLogs checks macOS >= 10.12 (Sierra), the minimum for unified logs.
func gateUnifiedLogs() error {
	version, err := macosVersion()
	if err != nil {
		return err
	}
	return gateVersion(version)
}

func gateVersion(v string) error {
	major, minor, err := parseVersion(v)
	if err != nil {
		return fmt.Errorf("parse macOS version %q: %w", v, err)
	}
	if major > 10 || (major == 10 && minor >= 12) {
		return nil
	}
	return fmt.Errorf("unified logs require macOS 10.12+, found %s", v)
}

func parseVersion(v string) (int, int, error) {
	parts := strings.Split(v, ".")
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("invalid version format")
	}
	var major, minor int
	if _, err := fmt.Sscanf(parts[0], "%d", &major); err != nil {
		return 0, 0, err
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minor); err != nil {
		return 0, 0, err
	}
	return major, minor, nil
}

func RegisterAll(registry *module.Registry) {
	customCleans := map[string]func(module.Context) error{
		"macos":      macosClean,
		"audit":      auditDarwinClean,
		"unified":    unifiedClean,
		"usage":      usageClean,
		"fileevents": fseventsClean,
	}

	modules := []struct {
		name string
		risk risk.RiskLevel
	}{
		{"shell", risk.RiskSafe},
		{"macos", risk.RiskRisky},
		{"audit", risk.RiskRisky},
		{"browser", risk.RiskRisky},
		{"unified", risk.RiskRisky},
		{"fileevents", risk.RiskRisky},
		{"usage", risk.RiskRisky},
	}

	for _, m := range modules {
		name := m.name
		artFn := func() []module.Artifact {
			arts, err := artifacts.ArtifactsForModule("darwin", name)
			if err != nil {
				return nil
			}
			return arts
		}
		registry.Register(module.NewYAMLModule(m.name, "darwin", m.risk, customCleans[m.name], artFn))
	}
}
