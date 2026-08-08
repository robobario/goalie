#!/usr/bin/env bash
set -euo pipefail
mkdir -p dist
CONTAINER_ENGINE=${CONTAINER_ENGINE:-docker}

if "${CONTAINER_ENGINE}" system info --format '{{.Host.ServiceIsRemote}}' 2>/dev/null | grep -q true; then
    # podman remote (e.g. macOS connecting to a podman machine): --output is not supported
    IMAGE_TAG="goalie-builder-tmp"
    "${CONTAINER_ENGINE}" build --target=builder -t "${IMAGE_TAG}" .
    CONTAINER_ID=$("${CONTAINER_ENGINE}" create "${IMAGE_TAG}")
    "${CONTAINER_ENGINE}" cp "${CONTAINER_ID}:/src/dist/." ./dist/
    "${CONTAINER_ENGINE}" rm "${CONTAINER_ID}"
else
    DOCKER_BUILDKIT=1 "${CONTAINER_ENGINE}" build --output=./dist --target=export .
fi

echo "Archives written to ./dist/"
