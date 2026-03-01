# Goodcommit

Goodcommit is a plugin-first CLI for generating structured, consistent commit messages.

It runs a configurable plugin pipeline across commit phases (`collect`, `enrich`, `finalize`, etc.), then composes the final message and executes `git commit`.

## Quick Start

1. Lock plugins:

```bash
go run ./cmd/goodcommit plugin lock \
  --plugins-config ./configs/goodcommit.plugins.json \
  --plugins-lockfile ./goodcommit.plugins.lock
```

2. Verify lockfile:

```bash
go run ./cmd/goodcommit plugin verify \
  --plugins-config ./configs/goodcommit.plugins.json \
  --plugins-lockfile ./goodcommit.plugins.lock
```

3. Run interactively:

```bash
go run ./cmd/goodcommit
```

4. Dry run (no commit):

```bash
go run ./cmd/goodcommit -m
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
go run ./cmd/goodcommit \
  --plugin-answer commit_type=feat \
  --plugin-answer commit_description="add plugin lock verification" \
  -m
```

To inspect required/optional answers for automation/agents:

```bash
go run ./cmd/goodcommit plugin context \
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

## License

MIT. See `LICENSE`.
