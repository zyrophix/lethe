package engine

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type ProcessInfo struct {
	Name string
	PID  string
}

func isProcessRunning(name string) bool {
	switch runtime.GOOS {
	case "linux", "darwin":
		out, err := exec.Command("pgrep", "-x", name).Output()
		if err != nil {
			return false
		}
		return strings.TrimSpace(string(out)) != ""
	case "windows":
		out, err := exec.Command("tasklist", "/FI", fmt.Sprintf("IMAGENAME eq %s", name), "/NH").Output()
		if err != nil {
			return false
		}
		return strings.Contains(string(out), name)
	default:
		return false
	}
}

func stopService(name string) error {
	switch runtime.GOOS {
	case "linux":
		if out, err := exec.Command("systemctl", "stop", name+".service").CombinedOutput(); err != nil {
			return fmt.Errorf("systemctl stop %s: %s", name, string(out))
		}
		return nil
	case "darwin":
		if out, err := exec.Command("launchctl", "stop", name).CombinedOutput(); err != nil {
			return fmt.Errorf("launchctl stop %s: %s", name, string(out))
		}
		return nil
	case "windows":
		if out, err := exec.Command("net", "stop", name).CombinedOutput(); err != nil {
			return fmt.Errorf("net stop %s: %s", name, string(out))
		}
		return nil
	default:
		return fmt.Errorf("unsupported platform for service management")
	}
}

func startService(name string) error {
	switch runtime.GOOS {
	case "linux":
		return exec.Command("systemctl", "start", name+".service").Run()
	case "darwin":
		return exec.Command("launchctl", "start", name).Run()
	case "windows":
		return exec.Command("net", "start", name).Run()
	default:
		return fmt.Errorf("unsupported platform")
	}
}

func serviceNameForProcess(procName string) string {
	switch procName {
	case "mysqld":
		return "mysql"
	case "auditd":
		return "auditd"
	case "apache2", "httpd":
		if runtime.GOOS == "linux" {
			return "apache2"
		}
		return "httpd"
	case "nginx":
		return "nginx"
	case "postgresql":
		return "postgresql"
	case "dockerd":
		return "docker"
	case "AeLookupSvc.exe":
		return "AeLookupSvc"
	default:
		return procName
	}
}
