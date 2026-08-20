package windows

import (
	"os"
	"os/exec"
	"strconv"
	"testing"

	"github.com/zyrophix/lethe/internal/module"
)

func TestUSNCleanDryRun(t *testing.T) {
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		t.Fatal("execCommand should not run in dry-run")
		return nil
	}
	defer func() { execCommand = orig }()

	if err := usnClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("usnClean dry-run: %v", err)
	}
}

func TestUSNCleanRunsCommand(t *testing.T) {
	var got [][]string
	calls := 0
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		got = append(got, append([]string{name}, args...))
		calls++
		// deletejournal succeeds; readjournal fails (journal gone).
		return fakeCmd(calls - 1)
	}
	defer func() { execCommand = orig }()

	if err := usnClean(module.Context{}); err != nil {
		t.Fatalf("usnClean: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 commands, got %v", got)
	}
	wantDelete := []string{"fsutil", "usn", "deletejournal", "/D", "C:"}
	wantRead := []string{"fsutil", "usn", "readjournal", "C:"}
	for i, cmd := range []struct{ got, want []string }{{got[0], wantDelete}, {got[1], wantRead}} {
		if len(cmd.got) != len(cmd.want) {
			t.Fatalf("cmd %d: got %v, want %v", i, cmd.got, cmd.want)
		}
		for j := range cmd.want {
			if cmd.got[j] != cmd.want[j] {
				t.Fatalf("cmd %d: got %v, want %v", i, cmd.got, cmd.want)
			}
		}
	}
}

func TestUSNCleanError(t *testing.T) {
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return fakeCmd(1)
	}
	defer func() { execCommand = orig }()

	if err := usnClean(module.Context{}); err == nil {
		t.Fatal("expected error from failing command")
	}
}

func TestUSNCleanVerifyJournalStillPresent(t *testing.T) {
	// deletejournal succeeds, readjournal succeeds -> journal still present.
	calls := 0
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		calls++
		return fakeCmd(0)
	}
	defer func() { execCommand = orig }()

	if err := usnClean(module.Context{}); err == nil {
		t.Fatal("expected error when journal still present")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestPagefileCleanDryRun(t *testing.T) {
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		t.Fatal("execCommand should not run in dry-run")
		return nil
	}
	defer func() { execCommand = orig }()

	if err := pagefileClean(module.Context{DryRun: true}); err != nil {
		t.Fatalf("pagefileClean dry-run: %v", err)
	}
}

func TestPagefileCleanRunsCommand(t *testing.T) {
	var got []string
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		got = append(got, append([]string{name}, args...)...)
		return fakeCmd(0)
	}
	defer func() { execCommand = orig }()

	_ = os.Setenv("SystemRoot", `C:\Windows`)
	defer func() { _ = os.Unsetenv("SystemRoot") }()

	if err := pagefileClean(module.Context{}); err != nil {
		t.Fatalf("pagefileClean: %v", err)
	}

	if len(got) < 2 || got[0] != "reg" {
		t.Fatalf("expected reg command first, got %v", got)
	}
}

func fakeCmd(exitCode int) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
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
