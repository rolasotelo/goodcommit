# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

Target release: `v0.3.2`

### Added

- GitHub Actions CI workflow for pushes, pull requests, and manual runs that executes the repository validation suite.
- Non-interactive CLI integration tests covering a simple built-in flow, a grouped flow with breaking-change metadata, and failure on missing required answers.

### Changed

- Plugin protocol compatibility is now enforced before execution, so manifests that do not advertise protocol `1.0` fail fast with a clear error.
- Permission flags and docs now explicitly describe plugin permissions as approval signals, not OS-level sandboxing.
- `plugin verify` now requires direct executable `source.type=path` plugins to be pinned in the lockfile. Older lockfiles for those plugins must be regenerated.

### Fixed

- Built-in text normalization now lowercases the first rune instead of the first byte, preserving UTF-8 commit text in description, body, why, and breaking-message plugins.
- `plugin lock` now builds local Go `source.type=path` plugins from the plugin's own module or package directory, even when the caller repository is not a Go module.
- Direct executable `source.type=path` plugins are now locked and verified using executable metadata instead of silently skipping integrity checks.

### Upgrade Notes

- Rebuild `goodcommit.plugins.lock` after upgrading:

  ```bash
  goodcommit plugin lock \
    --plugins-config ./configs/goodcommit.plugins.json \
    --plugins-lockfile ./goodcommit.plugins.lock
  ```

- If you maintain custom plugin manifests, ensure `protocol_versions` includes `"1.0"` until a newer negotiated protocol is supported end-to-end.
