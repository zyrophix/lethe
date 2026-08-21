// Package lethe is the public SDK for the lethe anti-forensics cleaner.
// It wraps the internal engine for embedding in other Go programs.
package lethe

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

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

// RiskLevel classifies how dangerous a cleaning operation is.
type RiskLevel int

const (
	RiskUndefined   RiskLevel = iota // 0 — unset, treated as RiskRisky
	RiskSafe                         // 1
	RiskRisky                        // 2
	RiskDestructive                  // 3
)

func (r RiskLevel) String() string {
	switch r {
	case RiskSafe:
		return "safe"
	case RiskRisky:
		return "risky"
	case RiskDestructive:
		return "destructive"
	default:
		return "unknown"
	}
}

// ParseRiskLevel parses "safe", "risky", "destructive".
func ParseRiskLevel(s string) (RiskLevel, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "safe":
		return RiskSafe, nil
	case "risky":
		return RiskRisky, nil
	case "destructive":
		return RiskDestructive, nil
	default:
		return RiskUndefined, fmt.Errorf("invalid risk level: %q (must be safe, risky, or destructive)", s)
	}
}

func toInternalRisk(r RiskLevel) risk.RiskLevel {
	switch r {
	case RiskSafe:
		return risk.RiskSafe
	case RiskDestructive:
		return risk.RiskDestructive
	case RiskRisky:
		return risk.RiskRisky
	default:
		return risk.RiskRisky // default for RiskUndefined
	}
}

func fromInternalRisk(r risk.RiskLevel) RiskLevel {
	switch r {
	case risk.RiskSafe:
		return RiskSafe
	case risk.RiskDestructive:
		return RiskDestructive
	default:
		return RiskRisky
	}
}

// EventLevel is the severity of a log event.
type EventLevel string

const (
	LevelInfo    EventLevel = "info"
	LevelSuccess EventLevel = "success"
	LevelWarning EventLevel = "warning"
	LevelError   EventLevel = "error"
	LevelDebug   EventLevel = "debug"
)

// Event is a structured log entry emitted by the engine.
type Event struct {
	Timestamp time.Time     `json:"ts"`
	Level     EventLevel    `json:"level"`
	Module    string        `json:"module,omitempty"`
	Artifact  string        `json:"artifact,omitempty"`
	Action    string        `json:"action,omitempty"`
	Risk      RiskLevel     `json:"-"`
	RiskStr   string        `json:"risk,omitempty"`
	Message   string        `json:"msg"`
	DryRun    bool          `json:"dry_run,omitempty"`
	Duration  time.Duration `json:"duration,omitempty"`
	Cleaned   int           `json:"cleaned,omitempty"`
	Failed    int           `json:"failed,omitempty"`
	Skipped   int           `json:"skipped,omitempty"`
	BackedUp  int           `json:"backed_up,omitempty"`
}

// Logger receives structured events from the cleaner.
// Implementations decide formatting and filtering.
type Logger interface {
	Log(Event)
}

// DiscardLogger drops all events.
type discardLogger struct{}

func (discardLogger) Log(Event) {}

// TextLogger writes human-readable lines to an io.Writer.
type TextLogger struct {
	mu  sync.Mutex
	out io.Writer
}

func NewTextLogger(out io.Writer) *TextLogger {
	if out == nil {
		out = io.Discard
	}
	return &TextLogger{out: out}
}

func (l *TextLogger) Log(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := e.Timestamp.Format(time.RFC3339)
	switch e.Level {
	case LevelSuccess:
		fmt.Fprintf(l.out, "[%s] [%s] %s %s (%s) %s\n", ts, e.Module, e.Action, e.Artifact, e.Risk.String(), e.Message)
	case LevelInfo:
		if e.Module != "" {
			fmt.Fprintf(l.out, "[%s] [%s] %s\n", ts, e.Module, e.Message)
		} else {
			fmt.Fprintf(l.out, "[%s] %s\n", ts, e.Message)
		}
	case LevelWarning:
		fmt.Fprintf(l.out, "[%s] [!] %s\n", ts, e.Message)
	case LevelError:
		fmt.Fprintf(l.out, "[%s] [-] %s\n", ts, e.Message)
	case LevelDebug:
		fmt.Fprintf(l.out, "[%s] [D] [%s] %s\n", ts, e.Module, e.Message)
	default:
		fmt.Fprintf(l.out, "[%s] [%s] %s\n", ts, e.Module, e.Message)
	}
}

// JSONLogger writes JSON lines to an io.Writer.
type JSONLogger struct {
	mu  sync.Mutex
	out io.Writer
}

func NewJSONLogger(out io.Writer) *JSONLogger {
	if out == nil {
		out = io.Discard
	}
	return &JSONLogger{out: out}
}

func (l *JSONLogger) Log(e Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if e.Risk != RiskUndefined {
		e.RiskStr = e.Risk.String()
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	data, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "json marshal error: %v\n", err)
		return
	}
	_, _ = l.out.Write(append(data, '\n'))
}

// AdvancedOptions groups optional, potentially destructive engine knobs.
// Nil means defaults (no backup, no parallel, no blocker killing).
type AdvancedOptions struct {
	Parallel     bool
	KillBlockers bool // maps to engine.Force — kills locking services
	Backup       *BackupOptions
}

// BackupOptions configures backup creation.
type BackupOptions struct {
	Dir string // empty means default (os.TempDir or /dev/shm)
}

// Options configures a clean run.
type Options struct {
	DryRun   bool
	UseShred bool
	MaxRisk  RiskLevel
	Modules  []string
	Logger   Logger    // nil = discard
	AuditLog io.Writer // optional JSON audit sink
	Advanced *AdvancedOptions
}

// Result summarizes a clean run.
type Result struct {
	Cleaned  int
	Failed   int
	Skipped  int
	BackedUp int
	Duration time.Duration
}

// loggerAdapter bridges public Logger to internal output.Writer.
type loggerAdapter struct {
	logger Logger
	audit  io.Writer
	mu     sync.Mutex
	dryRun bool
}

func newAdapter(logger Logger, audit io.Writer, dryRun bool) *loggerAdapter {
	if logger == nil {
		logger = discardLogger{}
	}
	return &loggerAdapter{logger: logger, audit: audit, dryRun: dryRun}
}

func (a *loggerAdapter) emit(e Event) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	e.DryRun = a.dryRun
	a.logger.Log(e)
	if a.audit != nil {
		// Audit is always JSON, includes RiskStr.
		if e.Risk != RiskUndefined {
			e.RiskStr = e.Risk.String()
		}
		data, err := json.Marshal(e)
		if err == nil {
			a.mu.Lock()
			_, _ = a.audit.Write(append(data, '\n'))
			a.mu.Unlock()
		}
	}
}

func (a *loggerAdapter) Info(module, msg string) {
	a.emit(Event{Level: LevelInfo, Module: module, Message: msg})
}

func (a *loggerAdapter) Success(module, artifact, action string, riskLevel risk.RiskLevel) {
	a.emit(Event{Level: LevelSuccess, Module: module, Artifact: artifact, Action: action, Risk: fromInternalRisk(riskLevel), Message: ""})
}

func (a *loggerAdapter) Warning(msg string) {
	a.emit(Event{Level: LevelWarning, Message: msg})
}

func (a *loggerAdapter) Error(msg string) {
	a.emit(Event{Level: LevelError, Message: msg})
}

func (a *loggerAdapter) Debug(module, msg string) {
	a.emit(Event{Level: LevelDebug, Module: module, Message: msg})
}

func (a *loggerAdapter) Summary(cleaned, failed, skipped, backedUp int, duration time.Duration, dryRun bool) {
	a.emit(Event{Level: LevelInfo, Message: "cleaning_complete", Cleaned: cleaned, Failed: failed, Skipped: skipped, BackedUp: backedUp, Duration: duration, DryRun: dryRun})
}

func (a *loggerAdapter) Flush() {}

// Clean runs the cleaner on the current platform.
func Clean(ctx context.Context, opts Options) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if opts.MaxRisk == RiskUndefined {
		opts.MaxRisk = RiskRisky
	}

	registry := newRegistry()
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return Result{}, fmt.Errorf("home directory: %w", err)
	}

	policy := risk.NewPolicy(toInternalRisk(opts.MaxRisk))
	adapter := newAdapter(opts.Logger, opts.AuditLog, opts.DryRun)
	eng := engine.New(registry, policy, adapter, homeDir)

	var adv AdvancedOptions
	if opts.Advanced != nil {
		adv = *opts.Advanced
	}
	var backupDir string
	var useBackup bool
	if adv.Backup != nil {
		useBackup = true
		backupDir = adv.Backup.Dir
	}

	// Map to engine RunOptions — CLI-only fields (Timestomp, StripXattr, WipeFreeSpace) are not exposed in SDK.
	runOpts := engine.RunOptions{
		DryRun:      opts.DryRun,
		UseShred:    opts.UseShred,
		UseBackup:   useBackup,
		BackupDir:   backupDir,
		Parallel:    adv.Parallel,
		Force:       adv.KillBlockers,
		ModuleNames: opts.Modules,
	}

	if err := eng.Run(ctx, runOpts); err != nil {
		return Result{}, err
	}

	stats := eng.GetStats()
	return Result{
		Cleaned:  stats.Cleaned,
		Failed:   stats.Failed,
		Skipped:  stats.Skipped,
		BackedUp: stats.BackedUp,
		Duration: stats.Duration,
	}, nil
}

// VerifyResult reports the verification status of a single artifact.
type VerifyResult struct {
	Path    string    `json:"path"`
	Method  string    `json:"method"`
	Risk    RiskLevel `json:"risk"`
	Cleaned bool      `json:"cleaned"`
	Reason  string    `json:"reason,omitempty"`
}

// VerifyOptions configures artifact verification.
type VerifyOptions struct {
	MaxRisk RiskLevel // RiskUndefined defaults to RiskRisky
	Modules []string  // empty means all platform modules
	// Logger receives one Event per artifact as verification progresses.
	Logger Logger
}

// Verify checks that forensic traces were cleaned. Returns true when all
// artifacts are clean. For per-artifact details use VerifyResults.
func Verify(ctx context.Context, maxRisk RiskLevel, modules []string) (bool, error) {
	results, err := VerifyResults(ctx, maxRisk, modules)
	if err != nil {
		return false, err
	}
	for _, r := range results {
		if !r.Cleaned {
			return false, nil
		}
	}
	return true, nil
}

// VerifyResults verifies artifacts and returns per-artifact results.
func VerifyResults(ctx context.Context, maxRisk RiskLevel, modules []string) ([]VerifyResult, error) {
	return VerifyResultsOpts(ctx, VerifyOptions{MaxRisk: maxRisk, Modules: modules})
}

// VerifyResultsOpts verifies artifacts with full options and streams each
// result to opts.Logger (if set) as events arrive.
func VerifyResultsOpts(ctx context.Context, opts VerifyOptions) ([]VerifyResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if opts.MaxRisk == RiskUndefined {
		opts.MaxRisk = RiskRisky
	}

	registry := newRegistry()
	policy := risk.NewPolicy(toInternalRisk(opts.MaxRisk))

	mods := registry.ListForPlatform(module.CurrentPlatform())
	if len(opts.Modules) > 0 {
		var filtered []module.Module
		for _, name := range opts.Modules {
			m, ok := registry.GetForPlatform(module.CurrentPlatform(), name)
			if !ok {
				return nil, fmt.Errorf("unknown module: %s", name)
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
		return nil, fmt.Errorf("home directory: %w", err)
	}

	internal := engine.VerifyAll(artifacts, platform.UserHomes(homeDir))
	results := make([]VerifyResult, 0, len(internal))
	for _, r := range internal {
		vr := VerifyResult{
			Path:    r.Artifact.Path,
			Method:  r.Artifact.Method,
			Risk:    fromInternalRisk(r.Artifact.GetRisk()),
			Cleaned: r.Cleaned,
			Reason:  r.Reason,
		}
		results = append(results, vr)

		if opts.Logger != nil {
			level := LevelSuccess
			if !vr.Cleaned {
				level = LevelWarning
			}
			opts.Logger.Log(Event{
				Timestamp: time.Now(),
				Level:     level,
				Artifact:  vr.Path,
				Action:    "verify",
				Risk:      vr.Risk,
				Message:   vr.Reason,
			})
		}
	}
	return results, nil
}

// ShredFile securely overwrites and removes a single file.
func ShredFile(ctx context.Context, path string, passes int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return shred.ShredFile(path, passes)
}

// Backup creates a backup archive of artifacts. Returns the archive path.
func Backup(ctx context.Context, dir string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}
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
func Restore(ctx context.Context, dir string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
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

// Ensure adapter satisfies output.Writer.
var _ output.Writer = (*loggerAdapter)(nil)
