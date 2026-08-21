# Contributing to Lethe

Thanks for your interest in improving Lethe — a cross-platform anti-forensics trace cleaner.

## Ground rules

- **Safety first.** Every change that touches cleaning behavior must preserve the risk model (`safe` / `risky` / `destructive`) and never auto-approve destructive operations.
- **Test everything.** New artifacts need catalog tests to pass; new engine/SDK behavior needs unit tests (`go test ./... -race`).
- **No malicious features.** PRs adding stealth, persistence, or anti-AV evasion will be rejected. Lethe erases traces; it does not hide itself.

## Development setup

```sh
git clone https://github.com/zyrophix/lethe
cd lethe
go build ./...        # Go 1.26+
go test ./... -race   # full suite
make vet
make build            # ./lethe binary (version injected via ldflags)
```

Integration and E2E suites:

```sh
go test ./internal/engine/ -tags=integration -race
make e2e              # Docker-based root-module E2E (linux)
```

## Adding artifacts

Artifacts live in `internal/artifacts/configs/artifacts/{linux,darwin,windows}.yaml`.

1. Add entries under the matching module with `path`, `method`, `risk`, and a `description`.
2. Use template variables: `{{.HomeDir}}`, `{{.AppData}}`, `{{.LocalAppData}}`, `{{.UserProfile}}`, `{{.Temp}}`, `{{.SystemRoot}}`, `{{.SystemDrive}}`.
3. Mark `backup: true` for every `destructive` artifact (enforced by tests).
4. Run `go test ./internal/artifacts/ -count=1` — duplicate paths, empty paths, unknown methods, and destructive-without-backup all fail CI.
5. Update the artifact counts in `README.md`.

## Adding custom cleans

Custom cleans are Go functions in `internal/modules/<platform>/`. They must:

- respect `ctx.DryRun` (no side effects in dry-run),
- run system commands through the injectable `execCommand(ctx.Ctx(), ...)` so cancellation works,
- return an error on failure.

## Commit style

Conventional Commits (`feat:`, `fix:`, `docs:`, `ci:`, `chore:`). Versioning follows SemVer for `0.x`: MINOR = user-visible features, PATCH = fixes/internal work. See `CHANGELOG.md`.

## Reporting issues

Open a GitHub issue with: OS/platform, `lethe version`, exact command, expected vs actual behavior, and (for crashes) the audit log or JSON output. Redact any sensitive paths before posting.
