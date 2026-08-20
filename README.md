# Lethe

Cross-platform anti-forensics trace cleaner with risk-gated operations. Written in Go as a safe, testable, and unique reimplementation of [evilsocket/nyx](https://github.com/evilsocket/nyx).

## Features

- **36 modules, 325 artifacts** across Linux (234), macOS (43), and Windows (48)
- **Risk gating**: every operation is classified `safe` / `risky` / `destructive` and filtered by `--max-risk`
- **Backup/restore**: tar archive with path-traversal protection before destructive changes
- **SQLite scrubbing**: DELETE + VACUUM, pure-Go driver (no CGO)
- **Windows registry wipe**, NTFS USN journal removal, pagefile cleaning
- **All-user cleaning**: expands `{{.HomeDir}}` across every real user home (`platform.UserHomes`)
- **SSD/CoW detection** with shred warnings (ZFS, Btrfs, F2FS, OCFS2)
- **Extended options**: secure shred, timestomp, free-space wipe, xattr stripping
- **Verify**: checks traces were actually cleaned (`lethe verify`, exit 1 on leftovers)
- **Dry-run, parallel modules, JSON output, audit log**
- **Go SDK** for embedding (`lethe.Clean`, `lethe.Verify`, ...)
- Static binary (~7.4MB), cross-compiled for 5 platforms, no dependencies

## Install

```sh
go install github.com/zyrophix/lethe@latest
# or a pinned version:
go install github.com/zyrophix/lethe@v0.2.0
```

Prebuilt binaries for Linux/macOS/Windows (amd64 + arm64) are attached to each [GitHub release](https://github.com/zyrophix/lethe/releases).

From source:

```sh
make build        # static binary at ./lethe
make cross-compile  # dist/lethe-<os>-<arch> for 5 platforms
```

Go 1.26+, Linux/macOS/Windows.

## Usage

Always start with a dry-run to preview changes:

```sh
lethe clean --dry-run --max-risk risky
lethe list                 # show modules + risk levels
lethe clean --force        # actually clean (skips confirmation)
```

### Clean options

| Flag | Meaning |
|------|---------|
| `-n, --dry-run` | preview changes without applying them |
| `-r, --max-risk` | `safe`, `risky` (default), or `destructive` |
| `-m, --modules` | comma-separated module list |
| `-p, --parallel` | run modules concurrently |
| `-b, --backup` | back up artifacts before cleaning |
| `-s, --shred` | secure-overwrite files before deletion |
| `--timestomp` | randomize timestamps after truncate |
| `--wipe-free-space` | fill free space to destroy deleted data |
| `--strip-xattr` | remove extended attributes |
| `-o, --output` | `text` or `json` |
| `--audit-log` | write an audit log to file |
| `-f, --force` | skip confirmation prompts |
| `-d, --debug` | verbose output |

### Verify

```sh
lethe verify --max-risk risky            # exit 0 if clean, 1 otherwise
lethe verify --modules browser,ssh -o json
```

### Restore

```sh
lethe clean --backup
lethe restore --backup-dir /path/to/backup
```

## Risk model

- **safe**: user data removal that is low risk (temp files, caches, logs)
- **risky**: forensic traces that can affect services if interrupted (audit, browser, network)
- **destructive**: irreversible operations (crypto-key removal, free-space wipe) that require confirmation

Operations above `--max-risk` are skipped, never auto-approved.

## Go SDK

```go
import "github.com/zyrophix/lethe"

res, err := lethe.Clean(lethe.Options{DryRun: true, MaxRisk: risk.RiskRisky})
ok, err := lethe.Verify(risk.RiskSafe, nil)
err := lethe.ShredFile("/tmp/secret", 3)
```

## Development

```sh
make test              # unit tests with -race
make test-integration  # integration tests (linux)
make vet
make cross-compile
make e2e               # Docker end-to-end against root modules (linux)
```

E2E (`docker/e2e.sh`) builds a hardened Ubuntu container (caps, memory/CPU/pids limits, memory sampler with abort below 2048MB free) and verifies that marked artifacts in `ssh`, `audit`, `logs`, and `temp` modules are removed. Full run is logged to `docker/e2e.log`.

CI (`.github/workflows/ci.yml`): test -race, cross-compile, integration, e2e-docker, golangci-lint.

## Safety notes

- Run as root for system-level modules; non-root users get home-dir cleaning only
- `--shred` on SSD/CoW filesystems is not guaranteed secure — a warning is shown (see `--strip-xattr` for metadata)
- Always back up (`--backup`) before destructive runs