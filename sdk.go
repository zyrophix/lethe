// Package lethe is the public SDK for the lethe anti-forensics cleaner.
// It wraps the internal engine for embedding in other Go programs.
package lethe

import (
	"fmt"
	"os"

	"github.com/zyrophix/lethe/internal/engine"
	"github.com/zyrophix/lethe/internal/module"
	"github.com/zyrophix/lethe/internal/modules/darwin"
	"github.com/zyrophix/lethe/internal/modules/linux"
	"github.com/zyrophix/lethe/internal/modules/windows"
	"github.com/zyrophix/lethe/internal/output"
	"github.com/zyrophix/lethe/internal/platform"
	"github.com/zyrophix/lethe/internal/risk"
	"github.com/zyrophix/lethe/internal/shred"
)

// Options configures a clean run.
type Options struct {
	DryRun        bool
	UseShred      bool
	Timestomp     bool
	WipeFreeSpace bool
	StripXattr    bool
	UseBackup     bool
	BackupDir     string
	Parallel      bool
	Force         bool
	MaxRisk       risk.RiskLevel
	Modules       []string
	Debug         bool
	Writer        output.Writer
}

// Result summarizes a clean run.
type Result struct {
	Cleaned  int
	Failed   int
	Skipped  int
	BackedUp int
}

// Clean runs the cleaner on the current platform.
func Clean(opts Options) (Result, error) {
	if opts.Writer == nil {
		opts.Writer = output.NewTextWriter(opts.Debug, "")
	}
	defer opts.Writer.Flush()

	if opts.MaxRisk == 0 {
		opts.MaxRisk = risk.RiskRisky
	}

	registry := newRegistry()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Result{}, fmt.Errorf("home directory: %w", err)
	}

	policy := risk.NewPolicy(opts.MaxRisk)
	eng := engine.New(registry, policy, opts.Writer, homeDir)

	if err := eng.Run(engine.RunOptions{
		DryRun:        opts.DryRun,
		UseShred:      opts.UseShred,
		Timestomp:     opts.Timestomp,
		WipeFreeSpace: opts.WipeFreeSpace,
		StripXattr:    opts.StripXattr,
		UseBackup:     opts.UseBackup,
		BackupDir:     opts.BackupDir,
		Parallel:      opts.Parallel,
		Force:         opts.Force,
		ModuleNames:   opts.Modules,
		Debug:         opts.Debug,
	}); err != nil {
		return Result{}, err
	}

	stats := eng.GetStats()
	return Result{
		Cleaned:  stats.Cleaned,
		Failed:   stats.Failed,
		Skipped:  stats.Skipped,
		BackedUp: stats.BackedUp,
	}, nil
}

// Verify checks that forensic traces were cleaned. Returns true when all
// artifacts are clean.
func Verify(maxRisk risk.RiskLevel, modules []string) (bool, error) {
	if maxRisk == 0 {
		maxRisk = risk.RiskRisky
	}

	registry := newRegistry()
	policy := risk.NewPolicy(maxRisk)

	mods := registry.ListForPlatform(module.CurrentPlatform())
	if len(modules) > 0 {
		var filtered []module.Module
		for _, name := range modules {
			m, ok := registry.GetForPlatform(module.CurrentPlatform(), name)
			if !ok {
				return false, fmt.Errorf("unknown module: %s", name)
			}
			filtered = append(filtered, m)
		}
		mods = filtered
	}

	var artifacts []module.Artifact
	for _, m := range mods {
		for _, a := range m.Artifacts() {
			if policy.Allowed(a.GetRisk()) {
				artifacts = append(artifacts, a)
			}
		}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false, fmt.Errorf("home directory: %w", err)
	}

	results := engine.VerifyAll(artifacts, platform.UserHomes(homeDir))
	for _, r := range results {
		if !r.Cleaned {
			return false, nil
		}
	}
	return true, nil
}

// ShredFile securely overwrites and removes a single file.
func ShredFile(path string, passes int) error {
	return shred.ShredFile(path, passes)
}

// Backup creates a backup archive of artifacts. Returns the archive path.
func Backup(dir string) (string, error) {
	registry := newRegistry()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home directory: %w", err)
	}

	var artifacts []module.Artifact
	for _, m := range registry.ListForPlatform(module.CurrentPlatform()) {
		artifacts = append(artifacts, m.Artifacts()...)
	}

	b := engine.NewBackup(dir)
	if err := b.Create(artifacts, homeDir); err != nil {
		return "", err
	}
	return b.Path(), nil
}

// Restore restores artifacts from a backup archive.
func Restore(dir string) error {
	b := engine.NewBackup(dir)
	return b.Restore()
}

func newRegistry() *module.Registry {
	r := module.NewRegistry()
	linux.RegisterAll(r)
	darwin.RegisterAll(r)
	windows.RegisterAll(r)
	return r
}
