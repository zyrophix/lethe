package engine

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	mrand "math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/output"
	"github.com/zyrophix/lethe/internal/platform"
	"github.com/zyrophix/lethe/internal/risk"
	"github.com/zyrophix/lethe/internal/shred"
	"github.com/zyrophix/lethe/internal/xattr"
)

type Stats struct {
	Cleaned  int
	Failed   int
	Skipped  int
	BackedUp int
	Duration time.Duration
	mu       sync.Mutex
}

func (s *Stats) IncCleaned()  { s.mu.Lock(); s.Cleaned++; s.mu.Unlock() }
func (s *Stats) IncFailed()   { s.mu.Lock(); s.Failed++; s.mu.Unlock() }
func (s *Stats) IncSkipped()  { s.mu.Lock(); s.Skipped++; s.mu.Unlock() }
func (s *Stats) IncBackedUp() { s.mu.Lock(); s.BackedUp++; s.mu.Unlock() }

type Engine struct {
	registry   *module.Registry
	policy     risk.Policy
	backup     *Backup
	writer     output.Writer
	stats      *Stats
	dryRun     bool
	useShred   bool
	timestomp  bool
	stripXattr bool
	force      bool
	debug      bool
	homeDir    string
	homes      []string
}

func New(registry *module.Registry, policy risk.Policy, writer output.Writer, homeDir string) *Engine {
	return &Engine{
		registry: registry,
		policy:   policy,
		writer:   writer,
		stats:    &Stats{},
		homeDir:  homeDir,
		homes:    platform.UserHomes(homeDir),
	}
}

// GetStats returns a copy of the current run statistics.
func (e *Engine) GetStats() Stats {
	e.stats.mu.Lock()
	defer e.stats.mu.Unlock()
	return Stats{
		Cleaned:  e.stats.Cleaned,
		Failed:   e.stats.Failed,
		Skipped:  e.stats.Skipped,
		BackedUp: e.stats.BackedUp,
		Duration: e.stats.Duration,
	}
}

type RunOptions struct {
	DryRun        bool
	UseShred      bool
	Timestomp     bool
	WipeFreeSpace bool
	StripXattr    bool
	UseBackup     bool
	BackupDir     string
	Parallel      bool
	Force         bool
	ModuleNames   []string
	Debug         bool
}

func (e *Engine) Run(opts RunOptions) error {
	start := time.Now()
	e.dryRun = opts.DryRun
	e.useShred = opts.UseShred
	e.timestomp = opts.Timestomp
	e.stripXattr = opts.StripXattr
	e.force = opts.Force
	e.debug = opts.Debug

	platformModules := e.registry.ListForPlatform(module.CurrentPlatform())

	var selected []module.Module
	if len(opts.ModuleNames) > 0 {
		platform := module.CurrentPlatform()
		for _, name := range opts.ModuleNames {
			m, ok := e.registry.GetForPlatform(platform, name)
			if !ok {
				return fmt.Errorf("unknown module: %s", name)
			}
			selected = append(selected, m)
		}
	} else {
		selected = platformModules
	}

	if len(selected) == 0 {
		return fmt.Errorf("no modules available for this platform")
	}

	var allArtifacts []module.Artifact
	for _, m := range selected {
		artifacts := m.Artifacts()
		for i := range artifacts {
			if e.policy.Allowed(artifacts[i].GetRisk()) {
				allArtifacts = append(allArtifacts, artifacts[i])
			} else {
				e.writer.Debug(m.Name(), fmt.Sprintf("skipping %s (risk=%s > max=%s)", artifacts[i].Path, artifacts[i].Risk, e.policy.MaxRisk))
				e.stats.IncSkipped()
			}
		}
	}

	if opts.UseShred {
		if w := platform.ShredWarning(); w != "" {
			e.writer.Warning(w)
		}
	}

	if opts.WipeFreeSpace && opts.DryRun {
		e.writer.Info("wipe", "would wipe free space (dry run)")
	}

	if opts.UseBackup {
		e.backup = NewBackup(opts.BackupDir)
		if !opts.DryRun {
			if err := e.backup.Create(allArtifacts, e.homeDir); err != nil {
				e.writer.Warning(fmt.Sprintf("backup failed: %v", err))
			} else {
				e.writer.Info("backup", "backup created at "+e.backup.Path())
			}
		}
	}

	if opts.Parallel {
		var wg sync.WaitGroup
		for _, m := range selected {
			wg.Add(1)
			go func(mod module.Module) {
				defer wg.Done()
				e.runModule(mod)
			}(m)
		}
		wg.Wait()
	} else {
		for _, m := range selected {
			e.runModule(m)
		}
	}

	if opts.WipeFreeSpace && !opts.DryRun {
		e.writer.Warning("wiping free space — this can take a long time")
		startWipe := time.Now()
		written, err := wipeFreeSpace(os.TempDir(), -1)
		if err != nil {
			e.writer.Error("wipe failed: " + err.Error())
		} else {
			e.writer.Info("wipe", fmt.Sprintf("wiped %d bytes of free space in %s", written, time.Since(startWipe)))
		}
	}

	e.stats.Duration = time.Since(start)
	e.writer.Summary(e.stats.Cleaned, e.stats.Failed, e.stats.Skipped, e.stats.BackedUp, e.stats.Duration, e.dryRun)

	return nil
}

func (e *Engine) runModule(m module.Module) {
	e.writer.Info(m.Name(), "cleaning module...")

	artifacts := m.Artifacts()
	for i := range artifacts {
		if !e.policy.Allowed(artifacts[i].GetRisk()) {
			e.stats.IncSkipped()
			continue
		}
		e.cleanArtifact(m.Name(), artifacts[i])
	}

	ctx := module.Context{
		DryRun:  e.dryRun,
		MaxRisk: e.policy.MaxRisk,
		HomeDir: e.homeDir,
		Debug:   e.debug,
		Backup:  e.backup != nil,
		Shred:   e.useShred,
	}
	if err := m.CustomClean(ctx); err != nil {
		e.writer.Error(fmt.Sprintf("[%s] custom clean failed: %v", m.Name(), err))
		e.stats.IncFailed()
	}
}

func (e *Engine) cleanArtifact(moduleName string, a module.Artifact) {
	for _, resolved := range e.resolvePaths(a.Path) {
		e.cleanResolvedPath(moduleName, a, resolved)
	}
}

func (e *Engine) resolvePaths(path string) []string {
	if !strings.Contains(path, "{{.HomeDir}}") {
		resolved := module.ResolvePath(path, e.homeDir)
		if resolved == "" {
			return nil
		}
		return []string{resolved}
	}

	var paths []string
	for _, home := range e.homes {
		resolved := module.ResolvePath(path, home)
		if resolved != "" {
			paths = append(paths, resolved)
		}
	}
	return paths
}

func (e *Engine) cleanResolvedPath(moduleName string, a module.Artifact, resolved string) {
	matches, err := filepath.Glob(resolved)
	if err != nil {
		e.writer.Debug(moduleName, fmt.Sprintf("invalid glob %s: %v", resolved, err))
		return
	}

	if len(matches) == 0 {
		if e.debug {
			e.writer.Debug(moduleName, "no match for "+resolved)
		}
		return
	}

	for _, m := range matches {
		if e.shouldExclude(m, a.Exclude) {
			e.stats.IncSkipped()
			continue
		}

		if len(a.LockingProcs) > 0 {
			var running []string
			for _, proc := range a.LockingProcs {
				if isProcessRunning(proc) {
					running = append(running, proc)
				}
			}
			if len(running) > 0 {
				if e.force {
					for _, proc := range running {
						svc := serviceNameForProcess(proc)
						e.writer.Debug(moduleName, fmt.Sprintf("stopping service %s (process %s)", svc, proc))
						if err := stopService(svc); err != nil {
							e.writer.Warning(fmt.Sprintf("failed to stop %s: %v", proc, err))
							e.stats.IncSkipped()
							continue
						}
						defer func(p, s string) {
							e.writer.Debug(moduleName, fmt.Sprintf("restarting service %s", s))
							startService(s)
						}(proc, svc)
					}
				} else {
					e.writer.Warning(fmt.Sprintf("skipping %s: locked by %s (use --force to stop)", m, strings.Join(running, ", ")))
					e.stats.IncSkipped()
					continue
				}
			}
		}

		if a.Backup && e.backup != nil && !e.dryRun {
			e.stats.IncBackedUp()
		}

		if e.dryRun {
			e.writer.Success(moduleName, m, "would "+a.Method, a.GetRisk())
			e.stats.IncCleaned()
			continue
		}

		method := a.GetMethod()
		if e.useShred && (method == module.MethodDelete) {
			method = module.MethodShred
		}

		var actionErr error
		switch method {
		case module.MethodTruncate:
			actionErr = e.truncateFile(m)
		case module.MethodDelete:
			actionErr = e.deletePath(m, a.Recursive)
		case module.MethodShred:
			actionErr = e.shredPath(m, a.Recursive)
		case module.MethodSQLite:
			actionErr = cleanSQLite(a, m, e.dryRun)
		case module.MethodWipeRegistry:
			actionErr = cleanRegistry(a, e.dryRun)
		case module.MethodSystemCommand:
			actionErr = e.runSystemCommand(a.Command)
		default:
			e.writer.Debug(moduleName, "unsupported method: "+a.Method)
			e.stats.IncSkipped()
			continue
		}

		if actionErr != nil {
			e.writer.Error(fmt.Sprintf("[%s] failed to %s %s: %v", moduleName, a.Method, m, actionErr))
			e.stats.IncFailed()
		} else {
			e.writer.Success(moduleName, m, a.Method, a.GetRisk())
			e.stats.IncCleaned()

			if e.stripXattr {
				if err := xattr.Clear(m); err != nil {
					if !os.IsNotExist(err) {
						e.writer.Debug(moduleName, fmt.Sprintf("xattr clear skipped for %s: %v", m, err))
					}
				}
			}
		}
	}
}

func (e *Engine) truncateFile(path string) error {
	if err := os.Truncate(path, 0); err != nil {
		return err
	}
	if e.timestomp {
		return timestompFile(path)
	}
	return nil
}

func timestompFile(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}

	now := time.Now()
	offset := time.Duration(mrand.Intn(15)-7) * 24 * time.Hour
	mtime := now.Add(offset)
	atime := now.Add(-time.Duration(mrand.Intn(15)) * 24 * time.Hour)
	return os.Chtimes(path, atime, mtime)
}

// wipeFreeSpace fills dir until the filesystem is out of space, then removes
// the filler file. maxBytes > 0 caps the write (used to emulate a small
// filesystem in tests). On SSD storage the filler is random; elsewhere zeros.
func wipeFreeSpace(dir string, maxBytes int64) (int64, error) {
	f, err := os.CreateTemp(dir, "lethe-wipe-*")
	if err != nil {
		return 0, err
	}
	filler := f.Name()
	defer os.Remove(filler)

	random := platform.SSD()
	buf := make([]byte, 1<<20)
	if random {
		for i := range buf {
			buf[i] = byte(mrand.Intn(256))
		}
	}

	var written int64
	for {
		chunk := buf
		if maxBytes > 0 {
			remaining := maxBytes - written
			if remaining <= 0 {
				break
			}
			if remaining < int64(len(buf)) {
				chunk = buf[:remaining]
			}
		}

		n, err := f.Write(chunk)
		written += int64(n)
		if err != nil {
			if isNoSpace(err) {
				break
			}
			return written, err
		}
	}

	if err := f.Close(); err != nil {
		return written, err
	}
	return written, nil
}

func isNoSpace(err error) bool {
	if errors.Is(err, syscall.ENOSPC) {
		return true
	}
	return strings.Contains(strings.ToLower(err.Error()), "no space left")
}

func (e *Engine) deletePath(path string, recursive bool) error {
	if recursive {
		return os.RemoveAll(path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return os.RemoveAll(path)
	}
	return os.Remove(path)
}

func (e *Engine) shredPath(path string, recursive bool) error {
	renamed, err := renameForShred(path)
	if err == nil {
		path = renamed
	} else {
		e.writer.Debug("shred", fmt.Sprintf("rename failed (%v), shredding original name", err))
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			subPath := filepath.Join(path, entry.Name())
			if !entry.IsDir() {
				if err := shred.ShredFile(subPath, 3); err != nil {
					return err
				}
			}
		}
		return os.RemoveAll(path)
	}
	return shred.ShredFile(path, 3)
}

func renameForShred(path string) (string, error) {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	temp := filepath.Join(filepath.Dir(path), "lethe-"+hex.EncodeToString(buf))
	return temp, osRename(path, temp)
}

var osRename = os.Rename

func (e *Engine) runSystemCommand(cmdStr string) error {
	parts := strings.Fields(cmdStr)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *Engine) shouldExclude(path string, patterns []string) bool {
	for _, p := range patterns {
		matched, _ := filepath.Match(p, filepath.Base(path))
		if matched {
			return true
		}
	}
	return false
}
