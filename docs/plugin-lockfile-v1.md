# Goodcommit Plugin Lockfile v1

The lockfile stores resolved plugin sources and checksums for reproducibility.

- File name: `goodcommit.plugins.lock`
- Format: JSON
- Version: `1`

## Example

```json
{
  "version": 1,
  "plugins": [
    {
      "id": "community/title-regex",
      "version": "v1.2.0",
      "source": {
        "type": "git",
        "repo": "https://github.com/acme/goodcommit-plugin-title-regex",
        "ref": "v1.2.0",
        "path": ".",
        "checksum": "sha256:..."
      },
      "manifest_checksum": "sha256:...",
      "executable_path": ".goodcommit/plugins/bin/community-title-regex",
      "executable_checksum": "sha256:...",
      "updated_at_utc": "2026-02-16T18:00:00Z"
    }
  ]
}
```

## Workflow

1. Install/resolve plugin source (`path`, `git`, or `builtin`) and build executable artifacts into `.goodcommit/plugins/bin`.
   Direct executable `path` plugins are pinned by checksum even when they are not rebuilt.
2. Read plugin manifest and compute checksum.
3. Upsert lockfile entry.
4. At runtime, verify source, manifest, and executable checksums before execution.
5. Refuse execution on mismatch unless user explicitly re-locks.

## Verification APIs

The `plugins` package includes helper functions:

- `ReadLockfile(path)`
- `WriteLockfile(path, lf)`
- `(*Lockfile).UpsertPlugin(p)`
- `FileSHA256(path)`
- `VerifyFileChecksum(path, expected)`
