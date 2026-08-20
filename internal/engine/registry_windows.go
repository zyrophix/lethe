//go:build windows

package engine

import (
	"fmt"
	"strings"

	"github.com/lethe/lethe/internal/module"
	"golang.org/x/sys/windows/registry"
)

func cleanRegistry(a module.Artifact, dryRun bool) error {
	keyPath := a.Path
	if dryRun {
		return nil
	}

	if strings.HasPrefix(keyPath, "HKCU:") {
		return deleteRegistryKey(registry.CURRENT_USER, strings.TrimPrefix(keyPath, "HKCU:\\"))
	} else if strings.HasPrefix(keyPath, "HKLM:") {
		return deleteRegistryKey(registry.LOCAL_MACHINE, strings.TrimPrefix(keyPath, "HKLM:\\"))
	}

	return fmt.Errorf("unsupported registry path: %s", keyPath)
}

func verifyRegistry(a module.Artifact, homeDir string) VerifyResult {
	keyPath := a.Path
	var root registry.Key
	var path string

	if strings.HasPrefix(keyPath, "HKCU:") {
		root = registry.CURRENT_USER
		path = strings.TrimPrefix(keyPath, "HKCU:\\")
	} else if strings.HasPrefix(keyPath, "HKLM:") {
		root = registry.LOCAL_MACHINE
		path = strings.TrimPrefix(keyPath, "HKLM:\\")
	} else {
		return VerifyResult{Artifact: a, Cleaned: false, Reason: "unsupported registry path"}
	}

	k, err := registry.OpenKey(root, path, registry.READ)
	if err != nil {
		return VerifyResult{Artifact: a, Cleaned: true, Reason: "registry key does not exist"}
	}
	k.Close()
	return VerifyResult{Artifact: a, Cleaned: false, Reason: "registry key still exists"}
}

func deleteRegistryKey(root registry.Key, path string) error {
	k, err := registry.OpenKey(root, path, registry.READ)
	if err != nil {
		return nil
	}
	defer k.Close()

	subkeys, err := k.ReadSubKeyNames(-1)
	if err == nil && len(subkeys) > 0 {
		for _, sk := range subkeys {
			registry.DeleteKey(root, path+`\`+sk)
		}
	}

	return registry.DeleteKey(root, path)
}
