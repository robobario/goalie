#!/usr/bin/env bash
# Regenerates compat test fixtures (plaintext or encrypted).
# Run this after a schema change, then commit data/ and export.golden.jsonl.
#
# Usage:
#   ./generate-data.sh --plaintext --output-dir tests/system/testdata/compat/plaintext-v1.0.0
#   ./generate-data.sh --encrypted --output-dir tests/system/testdata/compat/encrypted-v1.0.0
#
# Requires: git, rsync.
# GOALIE_BINARY env var may point to a pre-built binary; otherwise the binary
# is built from source.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
OUTPUT_DIR=""
ENCRYPTED=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --output-dir) OUTPUT_DIR="$2"; shift 2 ;;
        --plaintext)  ENCRYPTED=false; shift ;;
        --encrypted)  ENCRYPTED=true;  shift ;;
        --help)
            echo "Usage: $0 (--plaintext|--encrypted) --output-dir <dir>"
            exit 0 ;;
        *) echo "Unknown argument: $1"; exit 1 ;;
    esac
done

if [[ -z "$OUTPUT_DIR" ]]; then
    echo "Usage: $0 (--plaintext|--encrypted) --output-dir <dir>"
    exit 1
fi

GOALIE_BIN="${GOALIE_BINARY:-}"
if [[ -z "$GOALIE_BIN" ]]; then
    echo "Building goalie..."
    GOALIE_BIN="$(mktemp /tmp/goalie-gen-XXXXXX)"
    go build -o "$GOALIE_BIN" "$REPO_ROOT/cmd/goalie"
    trap 'rm -f "$GOALIE_BIN"' EXIT
fi

WORK_DIR="$(mktemp -d /tmp/goalie-compat-XXXXXX)"
trap 'rm -rf "$WORK_DIR"' EXIT

BARE_REPO="$WORK_DIR/bare.git"
GOALIE_HOME="$WORK_DIR/home"
GIT_HOME="$WORK_DIR/githome"

mkdir -p "$GIT_HOME" "$GOALIE_HOME"
cat > "$GIT_HOME/.gitconfig" <<'GITCFG'
[user]
    name = Test User
    email = test@example.com
[commit]
    gpgsign = false
GITCFG

git init --bare "$BARE_REPO"

goalie() {
    HOME="$GIT_HOME" GOALIE_HOME="$GOALIE_HOME" "$GOALIE_BIN" "$@"
}

tgoalie() {
    local ts="$1"; shift
    HOME="$GIT_HOME" GOALIE_HOME="$GOALIE_HOME" \
        GOALIE_FIXED_TIME_OVERRIDE="$ts" "$GOALIE_BIN" "$@"
}

if [[ "$ENCRYPTED" == "true" ]]; then
    printf 'y\nalice\n' | goalie init "file://$BARE_REPO"
else
    printf 'n\nalice\n' | goalie init "file://$BARE_REPO"
fi

tgoalie 2024-01-08T10:00:00Z goal add ROUTING "Implement the routing layer"
tgoalie 2024-01-08T10:01:00Z goal add SCALING "Improve scaling"
tgoalie 2024-01-08T10:02:00Z goal close SCALING

tgoalie 2024-01-08T10:00:00Z log "started the work"       --task "#impl" --goal ROUTING
tgoalie 2024-01-08T11:00:00Z log "blocked on code review" --task "#impl" --goal ROUTING --blocked

mkdir -p "$OUTPUT_DIR"

# Write a state file so the daily version check does not run during export,
# keeping the generated fixture free of version-recording side effects.
printf '{"last_version_check":"%s"}' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    > "$GOALIE_HOME/state.json"

goalie export > "$OUTPUT_DIR/export.golden.jsonl"

mkdir -p "$OUTPUT_DIR/data"
rsync -a --delete --exclude='.git' "$GOALIE_HOME/data/" "$OUTPUT_DIR/data/"

if [[ "$ENCRYPTED" == "true" ]]; then
    cp "$GOALIE_HOME/encryption.key" "$OUTPUT_DIR/test_encryption_key.hex"
    echo "Saved encryption key to $OUTPUT_DIR/test_encryption_key.hex"
fi

echo ""
echo "Generated $OUTPUT_DIR/export.golden.jsonl"
echo "Generated $OUTPUT_DIR/data/"
echo "NOTE: timestamps in data files are fixed; SHA256 of key-check.enc is"
echo "captured in the golden file. Commit everything together."
