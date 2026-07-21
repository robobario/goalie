#!/usr/bin/env bash
set -euo pipefail
mkdir -p dist
CONTAINER_ENGINE=${CONTAINER_ENGINE:-docker}
DOCKER_BUILDKIT=1 "${CONTAINER_ENGINE}" build --output=./dist --target=export .
echo "Binaries written to ./dist/"
