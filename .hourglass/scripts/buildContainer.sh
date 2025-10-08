#!/usr/bin/env bash
set -e

VERSION=""
IMAGE=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --tag)
      VERSION="$2"; shift 2;;
    --image)
      IMAGE="$2"; shift 2;;
    *)
      echo "Unknown option $1" >&2; exit 1;;
  esac
done

if [ -z "$IMAGE" ]; then
  echo "Error: --image is required" >&2
  exit 1
fi

if [ -n "$VERSION" ]; then
  fullImage="${IMAGE}:${VERSION}"
else
  fullImage="${IMAGE}:latest"
fi

echo "Pulling performer image: $fullImage" >&2
docker pull "$fullImage" >&2

IMAGE_REGISTRY=$(echo "$IMAGE" | awk -F/ '{print $1}')
IMAGE_DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' "$fullImage" | awk -F@ '{print $2}')

jq -n \
  --arg image "$fullImage" \
  --arg image_id "$IMAGE_ID" \
  --arg registry "$IMAGE_REGISTRY" \
  --arg digest "$IMAGE_DIGEST" \
  '{image: $image, image_id: $image_id, registry: $registry, digest: $digest}'

