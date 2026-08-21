# Lethe

[![CI](https://github.com/zyrophix/lethe/actions/workflows/ci.yml/badge.svg)](https://github.com/zyrophix/lethe/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/zyrophix/lethe)](https://github.com/zyrophix/lethe/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/zyrophix/lethe)](https://go.dev)
[![License](https://img.shields.io/github/license/zyrophix/lethe)](./LICENSE)
[![Go Report](https://goreportcard.com/badge/github.com/zyrophix/lethe)](https://goreportcard.com/report/github.com/zyrophix/lethe)

Cross-platform anti-forensics trace cleaner with risk-gated operations. Written in Go as a safe, testable, and unique reimplementation of [evilsocket/nyx](https://github.com/evilsocket/nyx).

> **Lethe** — river of oblivion in Greek mythology. The tool erases traces so forensics finds nothing.

## Features

- **37 modules, 358 artifacts** across Linux (236), macOS (49), and Windows (73)
- **Risk gating**: every operation is `safe` / `risky` / `destructive`, filtered by `--max-risk`
- **Backup/restore**: tar archive with path-traversal protection before destructive changes
- **SQLite scrubbing**: `DELETE` + `VACUUM`, pure-Go driver (no CGO)
- **Windows**: registry wipe (ShellBags, MUICache, RunMRU, ComDlg), NTFS USN journal, pagefile & shadow-copy (VSS) cleaning, Recycle Bin, Windows Search (`Windows.edb`), ETW traces
- **All-user cleaning**: expands `{{.HomeDir}}` across every real user home (`platform.UserHomes`)
- **SSD/CoW detection** with shred warnings (ZFS, Btrfs, F2FS, OCFS2)
- **Extended options**: secure shred, timestomp, free-space wipe, xattr stripping
- **Verify**: `lethe verify` checks traces are actually gone (exit 1 on leftovers)
- **Dry-run, parallel modules, JSON output, audit log**
- **Go SDK** (`lethe.Clean`, `lethe.Verify`, `lethe.ShredFile`, …)
- Static binary (~7.4 MB), 5 platforms, no dependencies

## Why Lethe?

| Tool | Language | Platforms | Artifacts | Safety |
|------|----------|-----------|-----------|--------|
| **nyx** (upstream) | bash + PowerShell | Linux/macOS/Windows | ~200+ | no backup/verify, no tests |
| **REDACT** | Python | Windows only | 255 | aggressive (RAM wipe, BitLocker header nuke) |
| **BleachBit** | Python | Linux/Windows | — | GUI cleaner, no forensics focus |
| **ShadowWipe / SATAN2** | C++/Rust | Windows / Linux+Win | — | covers VSS/shellbags **or** adds deception (fake logs) |
| **Lethe** | **Go** | **Linux/macOS/Windows** | **340** | **risk-gated + backup + verify + 180+ tests + CI** |

Lethe is the only cross-platform, statically-linked, test-covered cleaner with `safe/risky/destructive` gating, backup/restore and a verifiable `verify` step. It closes the Windows gap vs REDACT/ShadowWipe (VSS, Recycle Bin, ShellBags/BagMRU, MUICache, `Windows.edb`, ETW) without going into RAM-wiping or log-forging territory.

## Install

```sh
go install github.com/zyrophix/lethe/cmd/lethe@latest
# pinned version:
go install github.com/zyrophix/lethe/cmd/lethe@v0.5.0
```

Prebuilt binaries for Linux/macOS/Windows (amd64 + arm64) are attached to each [GitHub release](https://github.com/zyrophix/lethe/releases).

From source:

```sh
make build          # static binary at ./lethe
make cross-compile  # dist/lethe-<os>-<arch> for 5 platforms
```

Requires Go 1.26+.

## Quick start

Always preview first:

```sh
lethe list
lethe clean --dry-run --max-risk risky
lethe clean --force --max-risk risky --backup
lethe verify --max-risk risky   # exit 0 = clean, 1 = leftovers
```

## Usage

```
lethe clean   --dry-run --max-risk risky --modules shell,logs --parallel --shred --backup
lethe verify  --max-risk risky --modules browser,ssh -o json
lethe restore --backup-dir /path/to/backup.tar
lethe list
```

### Clean options

| Flag | Meaning |
|------|---------|
| `-n, --dry-run` | preview without applying |
| `-r, --max-risk` | `safe`, `risky` (default), or `destructive` |
| `-m, --modules` | comma-separated module list |
| `-p, --parallel` | run modules concurrently |
| `-b, --backup` | back up artifacts before cleaning |
| `-s, --shred` | secure-overwrite before delete |
| `--timestomp` | randomize timestamps after truncate |
| `--wipe-free-space` | fill free space to destroy deleted data |
| `--strip-xattr` | remove extended attributes |
| `-o, --output` | `text` or `json` |
| `--audit-log` | write audit log to file |
| `-f, --force` | skip confirmation |
| `-d, --debug` | verbose output |

### Verify & Restore

```sh
lethe verify --max-risk risky            # exit 0 if clean, 1 otherwise
lethe verify --modules browser,ssh -o json
lethe clean --backup
lethe restore --backup-dir /tmp/lethe-backup-*.tar
```

## Modules

| Platform | Modules | Artifacts |
|----------|---------|-----------|
| **Linux** (20) | `shell`, `logs`, `audit`, `temp`, `network`, `user`, `package`, `browser`, `ssh`, `container`, `systemd`, `print`, `cicd`, `idsips`, `crypto`, `privacy`, `pentest`, `osint`, `iot`, `ml` | 234 |
| **macOS** (7) | `shell`, `macos`, `audit`, `browser`, `unified`, `fileevents`, `usage` | 43 |
| **Windows** (10) | `events`, `history`, `registry`, `filesystem`, `temp`, `security`, `advanced`, `journal`, `pagefile`, `shadows` | 63 |

`events` = `wevtutil cl` for every log; `journal` = `fsutil usn deletejournal`; `pagefile` = `ClearPageFileAtShutdown` + delete; `shadows` = `vssadmin delete shadows /all /quiet`; `registry` includes ShellBags/BagMRU, MUICache, RunMRU, WordWheelQuery, TypedURLs, ComDlg MRUs, USBSTOR, BAM, ShimCache; `filesystem` includes Recycle Bin, `Windows.edb` (locked by `SearchIndexer.exe`), `hiberfil.sys`, ETW `RtBackup`/diagnostic logs, thumbcache.

Run `lethe list` for per-module risk levels.

## Risk model

- **safe** — low-risk caches/logs (temp files, thumbcache, history)
- **risky** — forensic traces that can affect services if interrupted (audit, browser, network, timeline)
- **destructive** — irreversible (ShimCache/BAM/USBSTOR, Amcache, VSS, free-space wipe); requires confirmation and supports `--backup`

Operations above `--max-risk` are skipped, never auto-approved.

## Go SDK

```go
import (
    "context"
    "github.com/zyrophix/lethe"
)

// Clean with risk gating and structured logging
res, err := lethe.Clean(context.Background(), lethe.Options{
    DryRun:  true,
    MaxRisk: lethe.RiskRisky,
    Logger:  lethe.NewTextLogger(os.Stdout),
    Advanced: &lethe.AdvancedOptions{
        Parallel: true,
        Backup: &lethe.BackupOptions{Dir: "/tmp/backup"},
    },
})
ok, err := lethe.Verify(context.Background(), lethe.RiskSafe, nil)
results, err := lethe.VerifyResults(context.Background(), lethe.RiskRisky, nil) // per-artifact detail
err = lethe.ShredFile(context.Background(), "/tmp/secret", 3)
archive, err := lethe.Backup(context.Background(), "/tmp")
err = lethe.Restore(context.Background(), "/tmp/backup.tar")
```

Risk levels are `lethe.RiskSafe` / `RiskRisky` / `RiskDestructive` (`RiskUndefined` defaults to `Risky`). `Logger` is `lethe.Logger` (`Log(Event)`) — use `lethe.NewTextLogger` or `lethe.NewJSONLogger`, with optional `AuditLog io.Writer`. See `sdk.go:1` for the full API.

## Development

```sh
make test              # unit tests with -race
make test-integration  # integration tests (linux, tag integration)
make vet
make cross-compile
make e2e               # Docker E2E against root modules (linux)
```

E2E (`docker/e2e.sh`) builds a hardened Ubuntu container (caps, memory/CPU/pids limits, memory sampler aborting below 2048 MB free) and verifies `ssh`, `audit`, `logs`, `temp` artifacts are removed. Log: `docker/e2e.log`.

CI (`.github/workflows/ci.yml`): `test -race` + `cross-compile` + `integration` + `e2e-docker` + `windows` + `golangci-lint v2`.

## Safety notes

- Run as root/Administrator for system-level modules; non-root gets home-dir cleaning only
- `--shred` on SSD/CoW is not guaranteed — a warning is shown (see `--strip-xattr` for metadata)
- Always `--backup` before `destructive` runs; restore with `lethe restore`
- Verify after cleaning: `lethe verify` is the source of truth

## Acknowledgements

Based on the artifact coverage of [evilsocket/nyx](https://github.com/evilsocket/nyx) (GPL-3.0). Lethe is a from-scratch Go reimplementation with a different engine (risk gating, backup, verify, SDK, CI).

## License

MIT — see [LICENSE](./LICENSE).
