# Goodcommit Plugin System v1

This document defines a robust, flexible plugin architecture for `goodcommit`.

## Goals

- Let users combine built-in plugins and community plugins.
- Keep the core stable while plugin behavior evolves.
- Support strict validators and flexible enrichers.
- Keep execution safe, observable, and deterministic.

## High-Level Architecture

This design is a plugin system with explicit extension points.

- Core engine owns workflow, UI, git write, and final commit execution.
- Plugins run as isolated executables (subprocesses), not in-process dynamic loading.
- Plugins communicate with core through a versioned JSON protocol over `stdin/stdout`.
- Built-in plugins use the same contract (or a compatible adapter) for consistency.

## Why subprocess plugins (instead of Go `plugin` package)

- Better portability across OS/toolchain combinations.
- Language-agnostic plugin development (Go, Python, Node, Rust, etc.).
- Stronger isolation and easier timeouts/resource controls.
- Easier community distribution.

## Plugin Lifecycle and Hook Phases

Core executes plugins in deterministic phase order:

1. `collect`

- Gather extra inputs (reasoning, ticket IDs, risk notes).
- Plugin may request declarative forms from core (`ui_requests`).

2. `validate`

- Lint/validate commit draft and optionally block commit.
- Example: title regex, typo threshold, prohibited words.

3. `enrich`

- Mutate or augment draft (AI rewrite, typo fixes, summary generation).

4. `finalize`

- Add trailers/signatures/metadata.
- Example: DCO signer, custom attestation trailers.

5. `pre_commit`

- Last gate before `git commit`.

6. `post_commit` (best effort)

- Non-blocking notifications/logging.

## Deterministic Ordering

Within each phase:

- Sort by `order` ascending.
- Tie-break by `id` lexicographically.

If a plugin returns `fatal=true`, remaining plugins in that phase are skipped and commit is blocked.

## Data Model

Core passes a single `CommitDraft` object through phases:

- `title`: string
- `body`: string
- `trailers`: array of `{key, value}`
- `metadata`: object (free-form key/value)
- `staged_files`: array of paths
- `git`: branch, repo root, HEAD, user name/email (if available)

Plugins can:

- Emit diagnostics (info/warn/error)
- Request declarative UI forms (`ui_requests`) rendered by core
- Return a patch-like mutation set
- Block commit with a reason

## Protocol Transport

- One JSON request per plugin invocation via `stdin`.
- One JSON response via `stdout`.
- `stderr` reserved for human-readable debug logs.
- Non-zero exit code = runtime/plugin failure.

Core should enforce:

- Per-plugin timeout (default 5s, configurable)
- Output size limits
- Optional memory/CPU limits (platform dependent)

## Compatibility Contract

- Versioned protocol: `protocol_version` (example: `"1.0"`).
- Plugin advertises supported versions in handshake.
- Core chooses highest mutually supported version.
- Unknown fields are ignored (forward compatibility).

## Security Model

Each plugin declares permissions in manifest:

- `network`: needed for AI or remote APIs
- `filesystem_read`: paths/globs
- `filesystem_write`: paths/globs
- `git_read`
- `git_write` (rare; default deny)
- `secrets`: names required (`OPENAI_API_KEY`, etc.)

Core policy:

- Deny by default unless user/organization policy allows.
- Show permission prompts for untrusted plugins.
- Persist trust decisions.
- Store plugin source checksums in a lock file.

## Distribution and Discovery

Supported plugin sources:

- `builtin`: bundled in core.
- `path`: local executable path.
- `git`: repo + ref + subpath + checksum.
- `registry` (future): centralized index.

Resolution is declared in project config, not hardcoded.

## Failure Semantics

Plugin failures are categorized:

- `PLUGIN_ERROR`: plugin crashed or returned invalid protocol.
- `POLICY_DENIED`: missing permission approval.
- `TIMEOUT`: execution exceeded timeout.
- `VALIDATION_BLOCK`: plugin intentionally blocked commit.

Per-plugin `failure_mode`:

- `fail_closed` (default for validate/finalize)
- `fail_open` (default for post_commit/enrich optional)

## Suggested Config Model

See `configs/plugins-config.example.json`.

Key ideas:

- Enable/disable per plugin.
- Optional `ui_group` to render multiple plugins' forms on one page (plugins with same group and contiguous order in a phase).
- Per-plugin runtime config blob (`config`) consumed by the plugin executable.
- Per-plugin timeout and failure mode.
- Optional `ai_instructions_append` and `ai_auto_answers` for local agent behavior.
- Manifest-owned fields (`hooks`, `entrypoint`, `permissions`, `ai_hints`, `contract`) are immutable from config.

## Suggested Manifest Model

See `docs/examples/plugin-manifest.example.json`.

Manifest is the source of truth for plugin protocol contract:

- `contract.answers`: expected answer keys/types for collectable inputs.
- `contract.metadata_reads`: metadata dependencies consumed from draft metadata.
- `contract.metadata_writes`: metadata keys produced by plugin mutations.
- `contract.trailer_writes`: trailers plugin may append in finalize/post phases.

## Example Plugin Mapping

- Reasoning plugin: `collect` + `enrich`
- Title regex checker: `validate`
- Typo checker: `validate` or `enrich`
- AI enricher: `enrich` with `network` permission
- Signer plugin: `finalize`

## Minimal v1 Request/Response

Schemas are provided at:

- `schemas/gpp-request.schema.json`
- `schemas/gpp-response.schema.json`
- `schemas/gpp-manifest.schema.json`
