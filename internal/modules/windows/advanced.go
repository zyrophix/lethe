package windows

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zyrophix/lethe/internal/module"
)

var execCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

func runCmd(ctx context.Context, name string, args ...string) error {
	out, err := execCommand(ctx, name, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return nil
}

// usnClean deletes the NTFS USN journal and verifies it via readjournal.
func usnClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := runCmd(ctx.Ctx(), "fsutil", "usn", "deletejournal", "/D", "C:"); err != nil {
		return err
	}
	if err := execCommand(ctx.Ctx(), "fsutil", "usn", "readjournal", "C:").Run(); err == nil {
		return fmt.Errorf("USN journal still present after delete")
	}
	return nil
}

// pagefileClean enables ClearPageFileAtShutdown and removes pagefile.sys.
func pagefileClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	if err := runCmd(ctx.Ctx(), "reg", "add",
		`HKLM\SYSTEM\CurrentControlSet\Control\Session Manager\Memory Management`,
		"/v", "ClearPageFileAtShutdown", "/t", "REG_DWORD", "/d", "1", "/f"); err != nil {
		return err
	}
	pagefile := filepath.Join(os.Getenv("SystemRoot"), "pagefile.sys")
	_ = os.Remove(pagefile)
	return nil
}
