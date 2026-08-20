package module

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/lethe/lethe/internal/platform"
)

func ResolvePath(path, homeDir string) string {
	distro := platform.Detect()

	replacements := map[string]string{
		"{{.HomeDir}}":         homeDir,
		"{{.AppData}}":         os.Getenv("APPDATA"),
		"{{.LocalAppData}}":    os.Getenv("LOCALAPPDATA"),
		"{{.UserProfile}}":     os.Getenv("USERPROFILE"),
		"{{.Temp}}":            os.TempDir(),
		"{{.SystemRoot}}":      os.Getenv("SystemRoot"),
		"{{.ApacheLogDir}}":    platform.ApacheLogDir(distro),
		"{{.PackageCacheDir}}": platform.PackageCacheDir(distro),
	}

	result := path
	for placeholder, value := range replacements {
		if !strings.Contains(result, placeholder) {
			continue
		}
		if value == "" && placeholder != "{{.Temp}}" {
			if runtime.GOOS == "windows" {
				return ""
			}
			result = strings.ReplaceAll(result, placeholder, homeDir)
		} else {
			result = strings.ReplaceAll(result, placeholder, value)
		}
	}

	if runtime.GOOS == "windows" {
		result = filepath.FromSlash(result)
	}

	return result
}

func CurrentPlatform() string {
	return runtime.GOOS
}
