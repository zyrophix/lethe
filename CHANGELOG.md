# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/zyrophix/lethe/compare/v0.3.1...HEAD
[0.3.1]: https://github.com/zyrophix/lethe/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/zyrophix/lethe/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/zyrophix/lethe/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/zyrophix/lethe/releases/tag/v0.1.0
