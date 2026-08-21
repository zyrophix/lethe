package engine

import (
	"archive/tar"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/platform"
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
		Dir:     filepath.Join(baseDir, fmt.Sprintf("lethe-backup-%d-%s", time.Now().Unix(), randToken())),
		created: time.Now(),
	}
}

// randToken returns a short random hex token so concurrent runs and
// local attackers cannot predict or pre-create the archive path.
func randToken() string {
	buf := make([]byte, 4)
	if _, err := crand.Read(buf); err != nil {
		// Fall back to time-based entropy; O_EXCL below still guards
		// against symlink attacks.
		return fmt.Sprintf("%x", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}

func (b *Backup) Path() string {
	return b.Dir + ".tar"
}

func (b *Backup) Create(artifacts []module.Artifact, homeDir string) error {
	return b.CreateForHomes(artifacts, platform.UserHomes(homeDir))
}

// CreateForHomes archives every existing backup-flagged path expanded
// over exactly the homes the engine will clean.
func (b *Backup) CreateForHomes(artifacts []module.Artifact, homes []string) error {
	paths := b.collectExistingPaths(artifacts, homes)
	if len(paths) == 0 {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(b.Path()), 0700); err != nil {
		return fmt.Errorf("create backup dir: %w", err)
	}

	// O_EXCL prevents symlink attacks: a pre-created
	// lethe-backup-*.tar -> /etc/shadow link makes os.Create truncate
	// the target. 0600 keeps archived secrets (keys, credentials)
	// private on shared machines.
	f, err := os.OpenFile(b.Path(), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("create backup file: %w", err)
	}
	defer func() { _ = f.Close() }()

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

// collectExistingPaths resolves backup-flagged artifacts across every
// real user home (matching how the engine expands {{.HomeDir}} for
// deletion), so nothing gets deleted without first entering the archive.
func (b *Backup) collectExistingPaths(artifacts []module.Artifact, homes []string) []string {
	var paths []string
	seen := make(map[string]bool)

	for _, a := range artifacts {
		if !a.Backup {
			continue
		}
		expandedHomes := homes
		if !strings.Contains(a.Path, "{{.HomeDir}}") {
			expandedHomes = []string{""}
		}
		if len(expandedHomes) == 0 {
			continue
		}
		for _, home := range expandedHomes {
			resolved := module.ResolvePath(a.Path, home)
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
