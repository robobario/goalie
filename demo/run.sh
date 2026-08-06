#!/usr/bin/env bash
# Produces demo/goalie-demo.gif by running VHS inside Docker.
# Requires: Docker, and dist/goalie-linux-amd64 built by build.sh.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ ! -f dist/goalie-linux-amd64 ]]; then
    echo "dist/goalie-linux-amd64 not found — run ./build.sh first" >&2
    exit 1
fi

CONTAINER_ENGINE=${CONTAINER_ENGINE:-docker}

"$CONTAINER_ENGINE" build -f demo/Dockerfile -t goalie-demo .
mkdir -p demo/output
"$CONTAINER_ENGINE" run --rm -v "$(pwd)/demo/output:/output:Z" goalie-demo

echo "GIF written to demo/output/goalie-demo.gif"
