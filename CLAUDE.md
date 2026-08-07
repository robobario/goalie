# Claude guidance for this repo

## Data schema changes

If you are modifying any file stored in the data repository — including JSON
field names or types, directory layout, file naming conventions, or the
encryption scheme — you must follow the schema versioning rules described in
[ARCHITECTURE.md](ARCHITECTURE.md).

Specifically:
- Determine whether the change is a PATCH, MINOR, or MAJOR bump using the
  table in ARCHITECTURE.md.
- Update `internal/schema/schema.go` with the new version.
- For MAJOR bumps, document the migration path in ARCHITECTURE.md before
  writing any code.
