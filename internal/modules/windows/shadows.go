package windows

import (
	"github.com/zyrophix/lethe/internal/module"
)

// shadowsClean deletes all Volume Shadow Copies via vssadmin.
// This is the Windows equivalent of shadow-copy forensics: each shadow
// copy preserves file versions that can survive normal deletion.
func shadowsClean(ctx module.Context) error {
	if ctx.DryRun {
		return nil
	}
	return runCmd(ctx.Ctx(), "vssadmin", "delete", "shadows", "/all", "/quiet")
}
