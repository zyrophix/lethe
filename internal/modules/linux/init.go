package linux

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/lethe/lethe/internal/artifacts"
	"github.com/lethe/lethe/internal/module"
	"github.com/lethe/lethe/internal/risk"
)

func shellClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	return exec.Command("bash", "-c", "history -c && unset HISTFILE").Run()
}

func logsClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	return exec.Command("journalctl", "--vacuum-size=1K", "--vacuum-time=1s").Run()
}

func auditClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := exec.Command("auditctl", "-D").Run(); err != nil {
		return fmt.Errorf("auditctl -D: %w", err)
	}
	if err := exec.Command("systemctl", "restart", "auditd").Run(); err != nil {
		return fmt.Errorf("restart auditd: %w", err)
	}
	return nil
}

func networkClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := exec.Command("ip", "neigh", "flush", "all").Run(); err != nil {
		return fmt.Errorf("flush arp: %w", err)
	}
	if err := exec.Command("nmcli", "connection", "reload").Run(); err != nil {
		return fmt.Errorf("nmcli reload: %w", err)
	}
	return nil
}

func sshClean(ctx module.Context) error {
	for _, logPath := range []string{"/var/log/auth.log", "/var/log/secure"} {
		if ctx.DryRun {
			out, err := exec.Command("grep", "-c", `sshd\[`, logPath).Output()
			if err == nil {
				count := strings.TrimSpace(string(out))
				return fmt.Errorf("would remove %s sshd entries from %s", count, logPath)
			}
			continue
		}
		if err := exec.Command("sed", "-i", `/sshd\[/d`, logPath).Run(); err != nil {
			return fmt.Errorf("sed %s: %w", logPath, err)
		}
	}
	return nil
}

func RegisterAll(registry *module.Registry) {
	customCleans := map[string]func(module.Context) error{
		"shell":   shellClean,
		"log":     logsClean,
		"audit":   auditClean,
		"network": networkClean,
		"ssh":     sshClean,
	}

	modules := []struct {
		name string
		risk risk.RiskLevel
	}{
		{"shell", risk.RiskSafe},
		{"logs", risk.RiskSafe},
		{"audit", risk.RiskRisky},
		{"temp", risk.RiskSafe},
		{"network", risk.RiskRisky},
		{"user", risk.RiskRisky},
		{"package", risk.RiskSafe},
		{"browser", risk.RiskRisky},
		{"ssh", risk.RiskRisky},
		{"container", risk.RiskRisky},
		{"systemd", risk.RiskRisky},
		{"print", risk.RiskSafe},
		{"cicd", risk.RiskRisky},
		{"idsips", risk.RiskRisky},
		{"crypto", risk.RiskDestructive},
		{"privacy", risk.RiskRisky},
		{"pentest", risk.RiskRisky},
		{"osint", risk.RiskRisky},
		{"iot", risk.RiskRisky},
		{"ml", risk.RiskSafe},
	}

	for _, m := range modules {
		name := m.name
		artFn := func() []module.Artifact {
			arts, err := artifacts.ArtifactsForModule("linux", name)
			if err != nil {
				return nil
			}
			return arts
		}
		registry.Register(module.NewYAMLModule(m.name, "linux", m.risk, customCleans[m.name], artFn))
	}
}
