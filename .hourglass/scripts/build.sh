#!/usr/bin/env bash
set -e

BUILD_CONTAINER=${BUILD_CONTAINER:-"true"}

if [[ "$BUILD_CONTAINER" == "true" ]]; then
    ./.hourglass/scripts/buildContainer.sh "$@"
else
    make build
fi
