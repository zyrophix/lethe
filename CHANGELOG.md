# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.5.1] - 2026-08-21

### Fixed
Security hardening from an independent code audit — all fixes are fail-closed:

- **Risk/method classification no longer fails open**: an artifact with a missing or invalid `risk:` is skipped with an explicit error instead of being treated as `safe`; an invalid `method:` is never coerced to `delete`
- **Custom cleans respect the risk policy**: module-level system commands (`auditctl`, `vssadmin`, `wevtutil`, …) do not run below the module's declared risk level
- **Backup failure aborts cleaning**: if the archive cannot be created, the run stops with an error before deleting anything (previously it warned and continued with exit 0)
- **Backup archive hardening**: created with `O_EXCL` and mode `0600` under an unpredictable random name — defeats symlink pre-creation attacks and local exposure of archived secrets
- **Multi-home backup coverage**: backup collection expands over exactly the user homes that will be cleaned, so nothing is deleted without first entering the archive
- **`recursive: false` is honored for directories**: non-recursive delete refuses directories instead of silently calling remove-all
- **Parallel cancellation waits** for in-flight workers before returning; shared stats/writer are no longer touched after `Run` ends
- **Verify inspects wildcarded SQLite paths** via glob expansion instead of reporting them "already clean"

### Changed
- Docs: badge set updated (GoReport service retired → pkg.go.dev), artifact counts synced across README/GitHub metadata (358), platform matrix clarified (5 targets, Windows amd64), `--backup-dir` documented
- Version reporting falls back to Go module build info, so binaries installed via `go install ...@v0.5.1` report their real version instead of "dev"

## [0.5.0] - 2026-08-21

### Added
- Cancellable custom cleans: `module.Context.StdCtx` (stdlib `context.Context`) propagated from the engine; all custom-clean system commands across Linux, macOS, and Windows now run via `exec.CommandContext` and abort on cancellation
- SDK: `VerifyOptions{MaxRisk, Modules, Logger}` + `VerifyResultsOpts` streaming per-artifact verification events (`success`/`warning`) to a `Logger` as they are produced
- Catalog expanded 340 → 358 artifacts: Windows Defender MPLog support logs, IIS LogFiles, `setupapi.dev.log`, LiveKernelReports, HTTPERR/CBS/DISM/WindowsUpdate/USOShared logs, WER Temp queue; macOS `.bash_sessions`, user `Library/Logs`, CrashReporter, sharedfilelist recents, Safari LastSession + binarycookies; Linux `.Xauthority`/`.ICEauthority`
- Godoc examples (`ExampleClean`, `ExampleVerifyResults`, `ExampleShredFile`)
- `CONTRIBUTING.md` and `SECURITY.md`

### Changed
- Platform `execCommand` test injectables now take `context.Context` as first argument
- `wipeFreeSpace` is an `Engine` method honoring context cancellation (since 0.4.0)

## [0.4.0] - 2026-08-21

### Added
- SDK: `VerifyResults(ctx, maxRisk, modules)` returns per-artifact verification results (`Path`, `Method`, `Risk`, `Cleaned`, `Reason`); `Verify` is now a thin wrapper over it
- Engine: deep cancellation — `wipeFreeSpace` aborts its write loop on `ctx.Done()`, system commands run via `exec.CommandContext`, directory shred checks context between files
- CI: Windows CLI smoke job (build + `list` + dry-run clean)

### Changed
- `wipeFreeSpace` is now a method on `Engine` taking `context.Context`

## [0.3.1] - 2026-08-21

### Fixed
- gofmt formatting of `Event` struct in the public SDK (CI lint failure on `main`)

### Changed
- Version is now injected at build time via `-ldflags` from a single source (`Makefile`) instead of being hardcoded in three places

## [0.3.0] - 2026-08-21

### Breaking (SDK)
- All SDK entry points now take `context.Context`: `Clean`, `Verify`, `ShredFile`, `Backup`, `Restore`
- `Options.MaxRisk` is now the public `lethe.RiskLevel` (`RiskUndefined=0` defaults to `Risky`; explicit `RiskSafe` no longer silently promoted) — previously leaked `internal/risk.RiskLevel`
- `Options.Writer output.Writer` replaced by `Logger Logger` + `AuditLog io.Writer`; new `Event`/`EventLevel` types and `TextLogger`/`JSONLogger` implementations
- `Force` renamed to `Advanced.KillBlockers`; `UseBackup`/`BackupDir` merged into `Advanced.Backup *BackupOptions`
- CLI-only options removed from the SDK: `WipeFreeSpace`, `Timestomp`, `StripXattr`, `Debug`
- `Result` now includes `Duration`

### Added
- Windows catalog expanded 48 → 63 artifacts: VSS shadow copies (`shadows` module), ShellBags/BagMRU, MUICache, RunMRU, WordWheelQuery, TypedURLs, LastVisitedPidlMRU, Recycle Bin, Windows Search (`Windows.edb`), `hiberfil.sys`, ETW traces, RDP bitmap cache
- `{{.SystemDrive}}` path template variable

### Changed
- Engine cancellation is cooperative between modules/artifacts via `context.Context`
- Repository metadata: MIT LICENSE, description, topics, README with badges and competitive comparison

## [0.2.0] - 2026-08-20

### Added
- Release pipeline (`.github/workflows/release.yml`): cross-compiled archives + SHA256SUMS on `v*` tags
- Windows CI job; platform tests made cross-platform (helper-process pattern)
- Artifact catalog expansion: VS Code, JetBrains, Vagrant paths across all platforms
- Public repository, `go install github.com/zyrophix/lethe/cmd/lethe@v0.2.0`

### Fixed
- golangci-lint v2 migration and errcheck exclusions
- Windows zip asset naming in release workflow

## [0.1.0] - 2026-08-20

### Added
- Initial release: Go reimplementation of evilsocket/nyx
- Risk-gated cleaning engine (`safe`/`risky`/`destructive` with `--max-risk`)
- 36 modules / 325 artifacts across Linux, macOS, Windows
- Backup/restore with path-traversal protection, SQLite scrubbing, verify, dry-run, parallel modules, JSON output, audit log
- Go SDK (`Clean`, `Verify`, `ShredFile`, `Backup`, `Restore`)
- Docker E2E harness, GitHub Actions CI, golangci-lint

[Unreleased]: https://github.com/zyrophix/lethe/compare/v0.5.1...HEAD
[0.5.1]: https://github.com/zyrophix/lethe/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/zyrophix/lethe/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/zyrophix/lethe/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/zyrophix/lethe/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/zyrophix/lethe/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/zyrophix/lethe/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/zyrophix/lethe/releases/tag/v0.1.0
