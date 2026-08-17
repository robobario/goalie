package schema

// Version is the current data schema version. Bump this when the layout or
// format of the data repository changes:
//   - major: breaking change, requires a migration step
//   - minor: backwards-compatible addition (new optional fields, new directories)
//   - patch: compatible fix with no structural change
const Version = "1.1.0"
