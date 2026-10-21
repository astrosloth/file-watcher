# Guidelines for AI Coding Agents

## Architecture

- `cmd/file-watcher`: CLI entrypoint (`install`, `uninstall`, `status`, foreground watcher).
- `pkg/config`: INI-style config parser with `~`, `$VAR`, and `%VAR%` path expansion.
- `pkg/watcher`: Directory watcher (`fsnotify` + polling fallback), per-file debounce timers, atomic cross-device move (`atomicCopyFile`).
- `pkg/pattern`: Brace expansion (`{a,b}`), glob matching, and regex capture/token target resolution (`{1}`, `$1`).
- `pkg/archive`: Single-file archive inspection and extraction (`.zip`, `.tar.*`, `.rar`, `.7z`).
- `pkg/service`: OS-specific background service management (`service_linux.go` systemd, `service_darwin.go` launchd, `service_windows.go` Startup shortcut + `wscript`).
- `pkg/logger`: Size-rotating file and console logger.

## Cross-Platform & Security Invariants

- **Platform parity**: `pkg/service/` uses OS build tags. When changing `pkg/service`, verify compilation across targets:
  `GOOS=linux go vet ./... && GOOS=darwin go vet ./... && GOOS=windows go vet ./...`
- **Script parity**: Keep `install.sh` / `install.ps1` and `check.sh` / `check.ps1` / `check.fish` aligned when updating install or check flows.
- **Atomic file writes**: Binary updates and cross-filesystem file moves must write to a `.tmp` file, `Sync()`, `Close()`, and `os.Rename()` to avoid partial writes or `ETXTBSY` on running executables.
- **Safe archive extraction**: Always enforce `MaxArchiveEntrySize` via bounded readers (`ExtractStreamBounded`) and sanitize entry filenames with `filepath.Base` to prevent decompression bombs and path traversal (Zip Slip).

## PR and Release Labeling

This repository uses automated release notes grouped by PR labels (configured in `.github/release.yml`). When opening PRs or managing branches, assign at least one category label:

| Label | Purpose | Release Section |
|---|---|---|
| `fix`, `bug`, `improvement` | Bug fixes, stability improvements, refactors | **Fixes & Improvements** |
| `breaking` | Incompatible API, CLI, or configuration changes | **Breaking Changes** |
| `documentation`, `docs` | README, guide, or doc comment updates | **Documentation** |
| `dependencies` | Upgrades to Go modules or GitHub Actions | **Dependencies** |
| `skip-changelog` | Trivial internal tweaks or CI chore not needing mention | *Excluded from release notes* |

## Git & PR Conventions

- **Branch naming**: `fix/...`, `feat/...`, `docs/...`, `chore/...`
- **Commit messages**: Conventional Commits style: `fix: ...`, `feat: ...`, `docs: ...`, `chore: ...`, `build(deps): ...`
- **Scope discipline**: Keep diffs minimal. Don't add speculative layers, interfaces, or unused dependencies.

## CI & Testing

- Formatting check: `gofmt -s -w .`
- Lint: `go vet ./...`
- Testing: `go test ./... -count=1`
- Race detection: `go test -race ./...` (requires CGO on Linux/macOS)
- Cross-OS compile check: `GOOS=windows go vet ./... && GOOS=darwin go vet ./...`
- Full local check script: `./check.sh` (or `./check.ps1` / `./check.fish`) runs `gofmt`, `go vet`, coverage, and builds `./file-watcher`.
