//go:build darwin

package xattr

import (
	"os"
	"os/exec"
)

// Clear removes all extended attributes from the file or symlink at path.
func Clear(path string) error {
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	cmd := exec.Command("xattr", "-c", path)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
