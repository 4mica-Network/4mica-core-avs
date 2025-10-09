#!/usr/bin/env bash
set -euo pipefail

# ------------------------------------------------------------------------------
# buildContainer.sh — build a Docker image for your AVS Performer or component
# Works with project structure:
#   4mica-core-avs/
#     ├── .hourglass/scripts/buildContainer.sh
#     ├── 4mica-core/Dockerfile
# ------------------------------------------------------------------------------

# Defaults
VERSION=""
IMAGE=""
DOCKERFILE="Dockerfile"
DOCKER_CONTEXT="."

# --- Parse command line arguments ---
while [[ $# -gt 0 ]]; do
  case $1 in
    --tag)
      VERSION="$2"
      shift 2
      ;;
    --image)
      IMAGE="$2"
      shift 2
      ;;
    --dockerfile)
      DOCKERFILE="$2"
      shift 2
      ;;
    --context)
      DOCKER_CONTEXT="$2"
      shift 2
      ;;
    *)
      echo "Unknown option: $1" >&2
      echo "Usage: $0 --image <name> [--tag <version>] [--dockerfile <path>] [--context <path>]" >&2
      exit 1
      ;;
  esac
done

# --- Validate image argument ---
if [[ -z "$IMAGE" ]]; then
  echo "Error: --image is required" >&2
  exit 1
fi

# --- Construct full image tag ---
if [[ -n "$VERSION" ]]; then
  fullImage="${IMAGE}:${VERSION}"
else
  fullImage="${IMAGE}"
fi

# --- Determine project root (two levels above this script) ---
SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
ROOT_DIR="$( cd "$SCRIPT_DIR/../.." && pwd )"

# --- Resolve Dockerfile ---
if [[ ! -f "$DOCKERFILE" ]]; then
  # Try relative to project root
  if [[ -f "$ROOT_DIR/$DOCKERFILE" ]]; then
    DOCKERFILE="$ROOT_DIR/$DOCKERFILE"
  elif [[ -f "$ROOT_DIR/4mica-core/Dockerfile" ]]; then
    DOCKERFILE="$ROOT_DIR/4mica-core/Dockerfile"
  else
    echo "Error: Could not find Dockerfile. Tried:" >&2
    echo "  $DOCKERFILE" >&2
    echo "  $ROOT_DIR/$DOCKERFILE" >&2
    echo "  $ROOT_DIR/4mica-core/Dockerfile" >&2
    exit 1
  fi
fi

# --- Resolve Docker context ---s
if [[ ! -d "$DOCKER_CONTEXT" ]]; then
  if [[ -d "$ROOT_DIR/4mica-core" ]]; then
    DOCKER_CONTEXT="$ROOT_DIR/4mica-core"
  elif [[ -d "$ROOT_DIR/$DOCKER_CONTEXT" ]]; then
    DOCKER_CONTEXT="$ROOT_DIR/$DOCKER_CONTEXT"
  else
    echo "Error: Could not find Docker build context. Tried:" >&2
    echo "  $DOCKER_CONTEXT" >&2
    echo "  $ROOT_DIR/4mica-core" >&2
    echo "  $ROOT_DIR/$DOCKER_CONTEXT" >&2
    exit 1
  fi
fi


# --- Convert to absolute paths ---
DOCKERFILE=$(cd "$(dirname "$DOCKERFILE")" && pwd)/$(basename "$DOCKERFILE")
# Resolve Docker context relative to the repo root (not this script’s folder)
if [[ ! "$DOCKER_CONTEXT" = /* ]]; then
  DOCKER_CONTEXT="$ROOT_DIR/$DOCKER_CONTEXT"
fi
DOCKER_CONTEXT=$(cd "$DOCKER_CONTEXT" && pwd)


# --- Sanity checks ---
if [[ ! -f "$DOCKERFILE" ]]; then
  echo "Error: Dockerfile not found at $DOCKERFILE" >&2
  exit 1
fi
if [[ ! -d "$DOCKER_CONTEXT" ]]; then
  echo "Error: Docker context directory not found at $DOCKER_CONTEXT" >&2
  exit 1
fi

# --- Show build info ---
echo "────────────────────────────────────────────"
echo "🧱  Building container"
echo "Image:        $fullImage"
echo "Dockerfile:   $DOCKERFILE"
echo "Context:      $DOCKER_CONTEXT"
echo "────────────────────────────────────────────"

# --- Run docker build ---
echo "Listing build context ($DOCKER_CONTEXT):"
ls -al "$DOCKER_CONTEXT"

docker build -f "$DOCKERFILE" -t "$fullImage" "$DOCKER_CONTEXT" >&2

# --- Get image ID ---
IMAGE_ID=$(docker images --format "{{.Repository}} {{.Tag}} {{.ID}}" \
  | grep "$IMAGE" | grep "${VERSION:-latest}" | awk '{print $3}' | tail -1)

if [[ -z "$IMAGE_ID" ]]; then
  echo "Error: Failed to retrieve image ID for $fullImage" >&2
  exit 1
fi

# --- Output result ---
echo "✅ Built container: $fullImage"
echo "🔹 Image ID: $IMAGE_ID"
echo "────────────────────────────────────────────"

# --- Return JSON result for higher-level scripts ---
jq -n \
  --arg image "$IMAGE" \
  --arg image_id "$IMAGE_ID" \
  '{image: $image, image_id: $image_id}'
