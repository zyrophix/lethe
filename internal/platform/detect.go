package platform

import (
	"runtime"
)

type Distro struct {
	Name    string
	Version string
	Family  string
}

func (d Distro) IsDebian() bool  { return d.Family == "debian" }
func (d Distro) IsRHEL() bool    { return d.Family == "rhel" }
func (d Distro) IsArch() bool    { return d.Family == "arch" }
func (d Distro) IsAlpine() bool  { return d.Family == "alpine" }
func (d Distro) IsDarwin() bool  { return d.Family == "darwin" }
func (d Distro) IsWindows() bool { return d.Family == "windows" }

func Detect() Distro {
	switch runtime.GOOS {
	case "linux":
		return detectLinux()
	case "darwin":
		return Distro{Name: "macos", Version: "unknown", Family: "darwin"}
	case "windows":
		return Distro{Name: "windows", Version: "unknown", Family: "windows"}
	default:
		return Distro{Name: runtime.GOOS, Version: "unknown", Family: runtime.GOOS}
	}
}

func Platform() string {
	return runtime.GOOS
}

func dedupeHomes(homes []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, h := range homes {
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		result = append(result, h)
	}
	return result
}
