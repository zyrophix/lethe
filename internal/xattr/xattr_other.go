//go:build !linux && !darwin

package xattr

// Clear is a no-op on platforms without extended attribute support.
func Clear(path string) error {
	return nil
}
