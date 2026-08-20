package module

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/lethe/lethe/internal/risk"
)

type CleanMethod int

const (
	MethodTruncate CleanMethod = iota
	MethodDelete
	MethodShred
	MethodWipeRegistry
	MethodSQLite
	MethodSystemCommand
)

func (m CleanMethod) String() string {
	switch m {
	case MethodTruncate:
		return "truncate"
	case MethodDelete:
		return "delete"
	case MethodShred:
		return "shred"
	case MethodWipeRegistry:
		return "wipe_registry"
	case MethodSQLite:
		return "sqlite"
	case MethodSystemCommand:
		return "system_command"
	default:
		return "unknown"
	}
}

func ParseCleanMethod(s string) (CleanMethod, error) {
	switch s {
	case "truncate":
		return MethodTruncate, nil
	case "delete":
		return MethodDelete, nil
	case "shred":
		return MethodShred, nil
	case "wipe_registry":
		return MethodWipeRegistry, nil
	case "sqlite":
		return MethodSQLite, nil
	case "system_command":
		return MethodSystemCommand, nil
	default:
		return MethodTruncate, fmt.Errorf("invalid clean method: %q", s)
	}
}

type Artifact struct {
	Path         string   `yaml:"path"`
	Risk         string   `yaml:"risk"`
	Method       string   `yaml:"method"`
	Backup       bool     `yaml:"backup"`
	LockingProcs []string `yaml:"locking_procs"`
	SQLiteTable  string   `yaml:"sqlite_table"`
	SQLiteWhere  string   `yaml:"sqlite_where"`
	Command      string   `yaml:"command"`
	Exclude      []string `yaml:"exclude"`
	Recursive    bool     `yaml:"recursive"`
	Description  string   `yaml:"description"`
}

func (a Artifact) GetRisk() risk.RiskLevel {
	rl, err := risk.ParseRiskLevel(a.Risk)
	if err != nil {
		return risk.RiskSafe
	}
	return rl
}

func (a Artifact) GetMethod() CleanMethod {
	m, err := ParseCleanMethod(a.Method)
	if err != nil {
		return MethodDelete
	}
	return m
}

type Context struct {
	DryRun    bool
	MaxRisk   risk.RiskLevel
	BackupDir string
	Backup    bool
	Shred     bool
	Parallel  bool
	HomeDir   string
	Debug     bool
}

type Module interface {
	Name() string
	Risk() risk.RiskLevel
	Platforms() []string
	Artifacts() []Artifact
	CustomClean(ctx Context) error
}

type Registry struct {
	mu      sync.RWMutex
	modules map[string]Module // key = "platform/name" e.g. "linux/shell"
}

func registryKey(platform, name string) string {
	return platform + "/" + name
}

func NewRegistry() *Registry {
	return &Registry{
		modules: make(map[string]Module),
	}
}

func (r *Registry) Register(m Module) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range m.Platforms() {
		r.modules[registryKey(p, m.Name())] = m
	}
}

func (r *Registry) GetForPlatform(platform, name string) (Module, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.modules[registryKey(platform, name)]
	return m, ok
}

func (r *Registry) ListForPlatform(platform string) []Module {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []Module
	seen := make(map[string]bool)
	for key, m := range r.modules {
		if !seen[m.Name()] && strings.HasPrefix(key, platform+"/") {
			result = append(result, m)
			seen[m.Name()] = true
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})
	return result
}
