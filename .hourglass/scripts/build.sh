#!/usr/bin/env bash
set -e

BUILD_CONTAINER=${BUILD_CONTAINER:-"false"}
make build 2>&1
if [[ "$BUILD_CONTAINER" == "true" ]]; then
    ./.hourglass/scripts/buildContainer.sh "$@"
fi

