# Guidelines for AI Coding Agents

Instructions and conventions for autonomous coding agents contributing to `file-watcher`.

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
