# Architecture

## Data repository layout

Goalie stores all shared state in a git repository (the "data repo"). Every
user clones the same branch. Concurrent writes from different machines are
reconciled with rebase-retry on push conflict.

```
data/
  meta.json           # repo-wide settings (encryption flag)
  key-check.enc       # encrypted token used to verify key correctness
  goals/              # one file per goal
    <GOAL_ID>.json
  journal/            # one JSONL file per (user, ISO week)
    <@username>-<YYYY>-W<WW>.jsonl
  motd/               # one file per message-of-the-day entry
    <timestamp>-<uuid>.txt
  versions/           # schema version tracking (see below)
    <uuid>.json
```

### Avoiding git conflicts

The file layout is designed so that concurrent writes from different users
rarely touch the same file:

- **Journal files** are keyed by `(username, ISO week)`. Two users always
  write to different files; one user writing two entries in the same week
  appends to the same file but resolves cleanly because pushes use
  rebase-retry (`git push` → on failure: `git pull --rebase` → retry).
- **Goals** are keyed by goal ID. Concurrent goal creation with different IDs
  produces no conflict.
- **MOTD and version files** use UUID-based filenames so concurrent writers
  never collide at the filesystem level. Logical consolidation (keeping only
  the latest MOTD, or only the highest version file) happens on the next
  write by whichever user "wins".

## Schema versioning

`internal/schema/schema.Version` is a `MAJOR.MINOR.PATCH` constant that
describes the format of everything stored in the data repo: directory layout,
file naming conventions, JSON field sets, and the encryption scheme.

### When to bump the schema version

| Change | Version component to bump |
|--------|--------------------------|
| New optional JSON field added to an existing file type | PATCH |
| New optional directory or file type added | MINOR |
| Existing field removed, renamed, or reinterpreted | MAJOR |
| File naming convention changed | MAJOR |
| Encryption scheme changed | MAJOR |
| Directory structure reorganised | MAJOR |

### How the version is propagated

Each journal entry carries `schema_version` so the provenance of every record
is recoverable from the raw data.

At runtime, goalie writes a UUID-named file to `versions/` containing the
current schema version. When the running version is the highest seen it
consolidates (deletes older files, writes one new file) so the directory stays
small. The highest-recorded major version is checked on startup and after every
`git pull`; if it exceeds the binary's supported major version goalie exits
immediately with an error rather than risk reading or writing incompatible data.

### Compatibility testing

`goalie export` dumps every entity in the data repository as JSONL — one
record per line, deterministic ordering, encryption-agnostic. The output
contains only data entities (goals, entries, MOTDs, version records); no
file layout details. Successful output proves the binary can decrypt and
parse the on-disk format.

Committed fixtures under `tests/system/testdata/compat/` cover plaintext
v1.0.0, pre-versioning (entries without `schema_version`), and encrypted
v1.0.0. The `tests/compat/generate-data.sh` script regenerates these
fixtures using `GOALIE_FIXED_TIME_OVERRIDE` for deterministic timestamps.

### How to perform a major version bump

1. Update `internal/schema/schema.go` with the new `MAJOR.MINOR.PATCH` value.
2. Update this document to describe what changed and what migration is required.
3. Write a migration tool or document manual migration steps for existing data
   repos before users upgrade.
4. Run `tests/compat/generate-data.sh` to regenerate compat fixtures for the
   new schema version and commit them alongside a new golden file.
5. Coordinate with all users: everyone must run the migration before any of
   them upgrades their binary, because the old binary will refuse to start
   once any peer has recorded the new major version.
