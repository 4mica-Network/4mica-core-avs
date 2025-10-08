#!/usr/bin/env bash
set -e

# Default image and version (can be overridden via env vars)
IMAGE_NAME=${IMAGE_NAME:-ghcr.io/4mica-network/4mica-core}
IMAGE_TAG=${IMAGE_TAG:-0.1.0-alpha.1}

# Compose expects PERFORMER_IMAGE
export PERFORMER_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"

echo "Using performer image: $PERFORMER_IMAGE"

# Optionally pull before running (to ensure local cache is up to date)
docker pull "$PERFORMER_IMAGE"

# Run docker compose
COMPOSE_PARALLEL_LIMIT=1 docker compose \
  --file ./.hourglass/docker-compose.yml \
  up
