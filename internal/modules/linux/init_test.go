package linux

import (
	"context"
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/zyrophix/lethe/internal/module"
)

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

func fakeCmd(exitCode int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
	cmd.Env = append(os.Environ(), "LETHE_HELPER_EXIT="+strconv.Itoa(exitCode))
	return cmd
}

func TestNetworkCleanRunsCommands(t *testing.T) {
	var got [][]string
	orig := execCommand
	execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
		got = append(got, append([]string{name}, args...))
		return fakeCmd(0)
	}
	defer func() { execCommand = orig }()

	if err := networkClean(module.Context{}); err != nil {
		t.Fatalf("networkClean: %v", err)
	}
	if len(got) != 2 || got[0][0] != "ip" || got[1][0] != "nmcli" {
		t.Errorf("unexpected commands: %v", got)
	}
}

func TestShellCleanCancelledContext(t *testing.T) {
	var gotCtx context.Context
	orig := execCommand
	execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if gotCtx == nil {
			gotCtx = ctx
		}
		cmd := fakeCmd(0)
		// Re-bind the helper to the propagated ctx so Run honors cancellation.
		cmd.Cancel = func() error { return context.Canceled }
		return exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	}
	defer func() { execCommand = orig }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := shellClean(module.Context{StdCtx: ctx}); err == nil {
		t.Fatal("expected error with cancelled context")
	}
	if gotCtx == nil {
		t.Fatal("execCommand did not receive StdCtx")
	}
	if gotCtx.Err() == nil {
		t.Error("expected propagated ctx to be cancelled")
	}
}

func TestAuditCleanStopsOnFirstError(t *testing.T) {
	calls := 0
	orig := execCommand
	execCommand = func(_ context.Context, name string, args ...string) *exec.Cmd {
		calls++
		return fakeCmd(1)
	}
	defer func() { execCommand = orig }()

	if err := auditClean(module.Context{}); err == nil {
		t.Fatal("expected error from failing auditctl")
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestLogsCleanDryRunNoCommands(t *testing.T) {
	orig := execCommand
	execCommand = func(context.Context, string, ...string) *exec.Cmd {
		t.Fatal("no commands expected in dry-run")
		return nil
	}
	defer func() { execCommand = orig }()

	if err := logsClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
}

func TestContextCtxFallback(t *testing.T) {
	var zero module.Context
	if zero.Ctx() == nil {
		t.Error("Ctx() should fall back to context.Background()")
	}
	custom, cancel := context.WithCancel(context.Background())
	defer cancel()
	mc := module.Context{StdCtx: custom}
	if mc.Ctx() != custom {
		t.Error("Ctx() should return StdCtx when set")
	}
}
