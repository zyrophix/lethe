package engine

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zyrophix/lethe/internal/module"
)

type Backup struct {
	Dir     string
	created time.Time
}

func NewBackup(baseDir string) *Backup {
	if baseDir == "" {
		if runtime.GOOS == "linux" {
			baseDir = "/dev/shm"
		} else {
			baseDir = os.TempDir()
		}
	}
	return &Backup{
		Dir:     filepath.Join(baseDir, fmt.Sprintf("lethe-backup-%d", time.Now().Unix())),
		created: time.Now(),
	}
}

func (b *Backup) Path() string {
	return b.Dir + ".tar"
}

func (b *Backup) Create(artifacts []module.Artifact, homeDir string) error {
	paths := b.collectExistingPaths(artifacts, homeDir)
	if len(paths) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(b.Path()), 0700); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	f, err := os.Create(b.Path())
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer f.Close()

	tw := tar.NewWriter(f)
	defer func() { _ = tw.Close() }()

	var archiveErrors []error
	for _, p := range paths {
		if err := b.addToArchive(tw, p); err != nil {
			archiveErrors = append(archiveErrors, fmt.Errorf("archive %s: %w", p, err))
		}
	}

	if len(archiveErrors) > 0 {
		return fmt.Errorf("backup completed with %d errors: %v", len(archiveErrors), archiveErrors[0])
	}
	return nil
}

func allowedRestorePrefixes() []string {
	if runtime.GOOS == "windows" {
		sd := os.Getenv("SystemDrive")
		if sd == "" {
			sd = "C:"
		}
		return []string{
			sd + `\`,
			os.TempDir() + `\`,
		}
	}
	return []string{
		"/home/",
		"/var/",
		"/etc/",
		"/tmp/",
		"/usr/",
		"/dev/shm/",
		"/root/",
		"/Users/",
		"/Library/",
		"/private/",
		"C:/",
	}
}

func sanitizeRestorePath(name string) (string, error) {
	if strings.Contains(name, "..") {
		return "", fmt.Errorf("path traversal rejected: %q", name)
	}
	cleaned := filepath.Clean(name)
	if runtime.GOOS != "windows" && !strings.HasPrefix(cleaned, "/") {
		cleaned = "/" + cleaned
	}
	allowed := false
	for _, prefix := range allowedRestorePrefixes() {
		if strings.HasPrefix(cleaned, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("path outside allowed prefixes: %q", name)
	}
	return cleaned, nil
}

func (b *Backup) Restore() error {
	path := b.Path()
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open backup: %w", err)
	}
	defer f.Close()

	tr := tar.NewReader(f)
	var restoreErrors []error

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup entry: %w", err)
		}

		target, err := sanitizeRestorePath(hdr.Name)
		if err != nil {
			restoreErrors = append(restoreErrors, fmt.Errorf("skipping unsafe path: %w", err))
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, os.FileMode(hdr.Mode)); err != nil {
				restoreErrors = append(restoreErrors, fmt.Errorf("mkdir %s: %w", target, err))
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				restoreErrors = append(restoreErrors, fmt.Errorf("mkdir %s: %w", filepath.Dir(target), err))
				continue
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				restoreErrors = append(restoreErrors, fmt.Errorf("create %s: %w", target, err))
				continue
			}
			if _, err := io.Copy(out, tr); err != nil {
				restoreErrors = append(restoreErrors, fmt.Errorf("write %s: %w", target, err))
			}
			out.Close()
		}
	}

	if len(restoreErrors) > 0 {
		return fmt.Errorf("restore completed with %d errors: %v", len(restoreErrors), restoreErrors[0])
	}
	return nil
}

func (b *Backup) Cleanup() error {
	return os.Remove(b.Path())
}

func (b *Backup) collectExistingPaths(artifacts []module.Artifact, homeDir string) []string {
	var paths []string
	seen := make(map[string]bool)

	for _, a := range artifacts {
		if !a.Backup {
			continue
		}
		resolved := module.ResolvePath(a.Path, homeDir)
		matches, err := filepath.Glob(resolved)
		if err != nil {
			continue
		}
		for _, m := range matches {
			if !seen[m] {
				seen[m] = true
				paths = append(paths, m)
			}
		}
	}

	return paths
}

func (b *Backup) addToArchive(tw *tar.Writer, path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return filepath.Walk(path, func(sub string, fi os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			return b.writeFile(tw, sub, fi)
		})
	}

	return b.writeFile(tw, path, info)
}

func (b *Backup) writeFile(tw *tar.Writer, path string, fi os.FileInfo) error {
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}
	hdr.Name = path

	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	if fi.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(tw, f)
	return err
}
