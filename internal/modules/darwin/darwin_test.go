package darwin

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/zyrophix/lethe/internal/module"
)

type cmdRecorder struct {
	cmds  [][]string
	fails int
}

func (r *cmdRecorder) fake() func(context.Context, string, ...string) *exec.Cmd {
	return func(_ context.Context, name string, args ...string) *exec.Cmd {
		r.cmds = append(r.cmds, append([]string{name}, args...))
		exit := 0
		if len(r.cmds) <= r.fails {
			exit = 1
		}
		return helperCmd(exit)
	}
}

func helperCmd(exitCode int) *exec.Cmd {
	return helperCmdCtx(context.Background(), exitCode)
}

// helperCmdCtx builds a helper process bound to ctx, so Run honors
// cancellation the same way CommandContext does in production code.
func helperCmdCtx(ctx context.Context, exitCode int) *exec.Cmd {
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(), "LETHE_HELPER_EXIT="+strconv.Itoa(exitCode))
	return cmd
}

func TestHelperProcess(t *testing.T) {
	if os.Getenv("LETHE_HELPER_EXIT") == "" {
		return
	}
	code, err := strconv.Atoi(os.Getenv("LETHE_HELPER_EXIT"))
	if err != nil {
		os.Exit(2)
	}
	os.Exit(code)
}

func rootAllowed() func() error {
	return func() error { return nil }
}

func TestMacosCleanDryRun(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := macosClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(rec.cmds) != 0 {
		t.Errorf("expected no commands in dry-run, got %v", rec.cmds)
	}
}

func TestMacosCleanCancelledContext(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	var gotCtx context.Context
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if gotCtx == nil {
			gotCtx = ctx
		}
		return helperCmdCtx(ctx, 0)
	}
	checkRoot = rootAllowed()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := macosClean(module.Context{StdCtx: ctx}); err == nil {
		t.Fatal("expected error with cancelled context")
	}
	if gotCtx == nil {
		t.Fatal("execCommand did not receive StdCtx")
	}
	if gotCtx.Err() == nil {
		t.Error("expected propagated ctx to be cancelled")
	}
}

func TestMacosCleanRunsCommands(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := macosClean(module.Context{}); err != nil {
		t.Fatalf("macosClean: %v", err)
	}
	if len(rec.cmds) != 4 {
		t.Fatalf("expected 4 commands, got %d: %v", len(rec.cmds), rec.cmds)
	}
	if rec.cmds[0][0] != "find" || rec.cmds[0][1] != "/" {
		t.Errorf("cmd[0] = %v, want find /", rec.cmds[0])
	}
	if rec.cmds[1][0] != "find" || rec.cmds[1][1] != "/Users/*/.Trash" {
		t.Errorf("cmd[1] = %v, want find /Users/*/.Trash", rec.cmds[1])
	}
	if rec.cmds[2][0] != "mdutil" {
		t.Errorf("cmd[2] = %v, want mdutil", rec.cmds[2])
	}
	if rec.cmds[3][0] != "qlmanage" {
		t.Errorf("cmd[3] = %v, want qlmanage", rec.cmds[3])
	}
}

func TestMacosCleanRequiresRoot(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	execCommand = (&cmdRecorder{}).fake()
	checkRoot = func() error { return errors.New("root required") }

	if err := macosClean(module.Context{}); err == nil {
		t.Fatal("expected root error")
	}
}

func TestMacosCleanCommandError(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{fails: 1}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := macosClean(module.Context{}); err == nil {
		t.Fatal("expected error from failing find")
	}
}

func TestAuditDarwinCleanDryRun(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := auditDarwinClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(rec.cmds) != 0 {
		t.Errorf("expected no commands in dry-run, got %v", rec.cmds)
	}
}

func TestAuditDarwinCleanRunsCommands(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := auditDarwinClean(module.Context{}); err != nil {
		t.Fatalf("auditDarwinClean: %v", err)
	}
	if len(rec.cmds) != 2 {
		t.Fatalf("expected 2 commands, got %v", rec.cmds)
	}
	if rec.cmds[0][0] != "audit" || rec.cmds[0][1] != "-t" {
		t.Errorf("cmd[0] = %v, want audit -t", rec.cmds[0])
	}
	if rec.cmds[1][0] != "audit" || rec.cmds[1][1] != "-s" {
		t.Errorf("cmd[1] = %v, want audit -s", rec.cmds[1])
	}
}

func TestAuditDarwinCleanCommandError(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{fails: 1}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := auditDarwinClean(module.Context{}); err == nil {
		t.Fatal("expected error from failing audit")
	}
}

func TestUnifiedCleanDryRun(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := unifiedClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(rec.cmds) != 0 {
		t.Errorf("expected no commands in dry-run, got %v", rec.cmds)
	}
}

func TestUnifiedCleanRunsErase(t *testing.T) {
	origExec, origRoot, origVersion := execCommand, checkRoot, macosVersion
	defer func() {
		execCommand, checkRoot, macosVersion = origExec, origRoot, origVersion
	}()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()
	macosVersion = func() (string, error) { return "13.2", nil }

	if err := unifiedClean(module.Context{}); err != nil {
		t.Fatalf("unifiedClean: %v", err)
	}
	if len(rec.cmds) != 1 || rec.cmds[0][0] != "log" || rec.cmds[0][1] != "erase" {
		t.Errorf("expected log erase --all, got %v", rec.cmds)
	}
}

func TestUnifiedCleanRejectsOldMacOS(t *testing.T) {
	origExec, origRoot, origVersion := execCommand, checkRoot, macosVersion
	defer func() {
		execCommand, checkRoot, macosVersion = origExec, origRoot, origVersion
	}()
	execCommand = (&cmdRecorder{}).fake()
	checkRoot = rootAllowed()
	macosVersion = func() (string, error) { return "10.11.6", nil }

	if err := unifiedClean(module.Context{}); err == nil {
		t.Fatal("expected version gate error on 10.11")
	}
}

func TestGateUnifiedLogsVersionError(t *testing.T) {
	origVersion := macosVersion
	defer func() { macosVersion = origVersion }()
	macosVersion = func() (string, error) { return "", errors.New("sw_vers failed") }

	if err := gateUnifiedLogs(); err == nil {
		t.Fatal("expected error from sw_vers failure")
	}
}

func TestUnifiedCleanVersionParseError(t *testing.T) {
	origExec, origRoot, origVersion := execCommand, checkRoot, macosVersion
	defer func() {
		execCommand, checkRoot, macosVersion = origExec, origRoot, origVersion
	}()
	execCommand = (&cmdRecorder{}).fake()
	checkRoot = rootAllowed()
	macosVersion = func() (string, error) { return "abc", nil }

	if err := unifiedClean(module.Context{}); err == nil {
		t.Fatal("expected parse error for malformed version")
	}
}

func TestUsageCleanDryRun(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()

	if err := usageClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(rec.cmds) != 0 {
		t.Errorf("expected no commands in dry-run, got %v", rec.cmds)
	}
}

func TestUsageCleanRunsDefaults(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()

	if err := usageClean(module.Context{}); err != nil {
		t.Fatalf("usageClean: %v", err)
	}
	if len(rec.cmds) != 3 {
		t.Fatalf("expected 3 defaults commands, got %v", rec.cmds)
	}
	for _, c := range rec.cmds {
		if c[0] != "defaults" || c[1] != "delete" || c[2] != "com.apple.recentitems" {
			t.Errorf("unexpected command: %v", c)
		}
	}
}

func TestUsageCleanError(t *testing.T) {
	origExec := execCommand
	defer func() { execCommand = origExec }()
	rec := &cmdRecorder{fails: 1}
	execCommand = rec.fake()

	if err := usageClean(module.Context{}); err == nil {
		t.Fatal("expected error from failing defaults delete")
	}
}

func TestFseventsCleanDryRun(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := fseventsClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(rec.cmds) != 0 {
		t.Errorf("expected no commands in dry-run, got %v", rec.cmds)
	}
}

func TestFseventsCleanRunsFind(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	rec := &cmdRecorder{}
	execCommand = rec.fake()
	checkRoot = rootAllowed()

	if err := fseventsClean(module.Context{}); err != nil {
		t.Fatalf("fseventsClean: %v", err)
	}
	if len(rec.cmds) != 1 {
		t.Fatalf("expected 1 find command, got %v", rec.cmds)
	}
	joined := strings.Join(rec.cmds[0], " ")
	if !strings.Contains(joined, ".fseventsd") {
		t.Errorf("find command should target .fseventsd: %v", rec.cmds[0])
	}
}

func TestFseventsCleanRequiresRoot(t *testing.T) {
	origExec, origRoot := execCommand, checkRoot
	defer func() { execCommand, checkRoot = origExec, origRoot }()
	execCommand = (&cmdRecorder{}).fake()
	checkRoot = func() error { return errors.New("root required") }

	if err := fseventsClean(module.Context{}); err == nil {
		t.Fatal("expected root error")
	}
}
