# Rework Tracker

This tracks implementation progress for the robustness refactor items identified in the March 1, 2026 review.

## Status

- [x] P1: Lockfile portability for default `gobin` (`executable_path` portability)
- [x] P1: Builtin binary collision risk in shared `GOBIN`
- [x] P1: Deterministic install/build (remove `@latest` fallback drift)
- [ ] P1: Stronger plugin isolation/sandboxing model
- [x] P2: Explicit git-context error handling (avoid silent metadata fallbacks)
- [x] P2: Split `/Users/rolandosotelo/dev/rolasotelo/goodcommit/cmd/goodcommit/main.go` into focused files
- [x] P2: Add automated tests for lock/runtime/UI parsing critical paths
- [x] P3: `init` robustness (clear partial-failure behavior and stronger guidance)

## Notes

- This tracker is intentionally scoped to the findings from the last review.
- Items are checked only after implementation + validation (`go test ./...`, `go build ./...`).
