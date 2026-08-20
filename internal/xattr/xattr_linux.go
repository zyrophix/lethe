//go:build linux

package xattr

import (
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// Clear removes all extended attributes from the file or symlink at path.
// Returns an error only if attributes exist but could not be removed.
func Clear(path string) error {
	if _, err := os.Lstat(path); err != nil {
		return err
	}

	names, err := list(path)
	if err != nil || len(names) == 0 {
		return nil
	}

	for _, name := range names {
		if err := unix.Lremovexattr(path, name); err != nil {
			return err
		}
	}
	return nil
}

func list(path string) ([]string, error) {
	// Llistxattr with a zero buffer returns the required size.
	size, err := unix.Llistxattr(path, nil)
	if err != nil {
		return nil, err
	}
	if size == 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	size, err = unix.Llistxattr(path, buf)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, name := range strings.Split(string(buf[:size]), "\x00") {
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}
