# Security Policy

## Supported versions

| Version | Supported |
|---------|-----------|
| latest release on GitHub | yes |
| older releases | no — upgrade |

## Reporting a vulnerability

**Do not open a public issue for security vulnerabilities.**

Use GitHub's private vulnerability reporting: *Security → Report a vulnerability* on [zyrophix/lethe](https://github.com/zyrophix/lethe/security), or contact the maintainer directly.

Include: affected version (`lethe version`), platform, impact description, and reproduction steps. You will get an initial response within 7 days.

## Scope

In scope:
- Path traversal or unsafe restore in backup/restore
- Risk-gating bypass (destructive operations running below their declared risk)
- Deletion outside declared artifact paths (over-broad globs, template expansion bugs)
- Injection via artifact YAML into system commands

Out of scope:
- The tool's intended destructive behavior when explicitly run with elevated risk levels
- Forensic recoverability of deleted data on CoW/SSD filesystems (documented limitation)

## Design notes for auditors

- Destructive artifacts require explicit `--max-risk destructive` and interactive `I UNDERSTAND` confirmation
- Backup restore rejects paths outside allow-listed prefixes and any `..` components (`internal/engine/backup.go`)
- Custom cleans run fixed argument vectors via `exec.CommandContext`; no shell interpolation of user input
