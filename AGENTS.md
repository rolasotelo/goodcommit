# AGENTS.md

This file defines how coding agents should work in this repository.

## Scope

- Applies to the entire repository.
- If a deeper `AGENTS.md` exists in a subdirectory, the deeper file overrides this one for that subtree.

## Project Overview

- This repo is a plugin-first commit message system.
- Core CLI: `cmd/goodcommit`.

## Engineering Standards

- Prefer deterministic behavior and explicit errors.
- Preserve backward compatibility unless change is intentional and documented.

## Validation Before Commit

Run these before committing unless the task explicitly says otherwise:

```bash
go test ./...
go build ./...
```

## Commit Quality Rules

- Commit message must reflect actual staged diff.
- Do not claim behavior not present in code.
- Prefer small, coherent commits.
- If tests were not run, state that explicitly in handoff.
