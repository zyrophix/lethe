//go:build !windows

package engine

import (
	"fmt"

	"github.com/lethe/lethe/internal/module"
)

func cleanRegistry(a module.Artifact, dryRun bool) error {
	if dryRun {
		return nil
	}
	return fmt.Errorf("registry cleaning is only available on Windows")
}

func verifyRegistry(a module.Artifact, homeDir string) VerifyResult {
	return VerifyResult{Artifact: a, Cleaned: false, Reason: "registry cleaning is only available on Windows"}
}
