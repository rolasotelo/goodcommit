# Goodcommit

Goodcommit is a plugin-first CLI for generating structured, consistent commit messages.

It runs a configurable plugin pipeline across commit phases (`collect`, `enrich`, `finalize`, etc.), then composes the final message and executes `git commit`.

## Quick Start

1. Install:

```bash
go install github.com/rolasotelo/goodcommit/cmd/goodcommit@latest
```

2. Bootstrap config in your repository:

```bash
goodcommit init
```

This scaffolds:

- `configs/goodcommit.plugins.json`
- `configs/commit-types.json`
- `goodcommit.plugins.lock` (unless `--lock=false`)

3. Run interactively:

```bash
goodcommit
```

4. Dry run (no commit):

```bash
goodcommit -m
```

## Existing Repo Setup

If config already exists and you only want to refresh lock/build artifacts:

```bash
goodcommit plugin lock \
  --plugins-config ./configs/goodcommit.plugins.json \
  --plugins-bin-dir gobin \
  --plugins-lockfile ./goodcommit.plugins.lock
```

Verify:

```bash
goodcommit plugin verify \
  --plugins-config ./configs/goodcommit.plugins.json \
  --plugins-lockfile ./goodcommit.plugins.lock
```

## Default Built-in Flow

Current default stack (from `configs/goodcommit.plugins.json`):

- `builtin/logo`
- `builtin/types`
- `builtin/scopes`
- `builtin/description`
- `builtin/why`
- `builtin/body`
- `builtin/breaking`
- `builtin/breakingmsg`
- `builtin/coauthors`
- `builtin/conventional-title`
- `builtin/signedoffby`

Notes:

- `logo` and `types` share one grouped page (`ui_group: "intro"`).
- Grouped UI is compact by default.
- Use `--detailed-ui` (or `GOODCOMMIT_DETAILED_UI=true`) to show verbose grouped section headings/instructions.

## Non-Interactive Usage

You can provide answers via `--plugin-answer`:

```bash
goodcommit \
  --plugin-answer commit_type=feat \
  --plugin-answer commit_description="add plugin lock verification" \
  -m
```

To inspect required/optional answers for automation/agents:

```bash
goodcommit plugin context \
  --plugins-config ./configs/goodcommit.plugins.json \
  --plugins-lockfile ./goodcommit.plugins.lock
```

## Config Notes

- Main plugin config: `configs/goodcommit.plugins.json`
- Example config: `configs/plugins-config.example.json`
- Default logo file: `configs/logo.ascii.txt`
- Scope options file: `configs/commit-scopes.json`
- Co-author options file: `configs/commit-coauthors.json`

Built-in plugins can omit explicit `manifest`/`source` in config; runtime resolves them from embedded built-in definitions.

You can force specific plugin answer keys to be mandatory via `required_answers` on that plugin config entry. Example:

```json
{
  "id": "builtin/body",
  "required_answers": ["commit_body"]
}
```

This affects both:
- `goodcommit plugin context` output (`commit_body` will move to `required_answers`)
- Runtime execution (plugin invocation fails if required answers are still missing/empty, even when that plugin uses `fail_open`)

## `init` Flags

```bash
goodcommit init [flags]
```

- `--plugins-config` path to scaffold plugins config (default `./configs/goodcommit.plugins.json`)
- `--types-config` path to scaffold commit types config (default `./configs/commit-types.json`)
- `--plugins-lockfile` lockfile output path (default `goodcommit.plugins.lock`)
- `--plugins-bin-dir` executable output directory (default `gobin`, which resolves to `GOBIN`/Go bin dir)
- `--lock` build+write lockfile after scaffolding (default `true`)
- `--force` overwrite scaffold files if they already exist

## License

MIT. See `LICENSE`.
