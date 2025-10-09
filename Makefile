# -----------------------------------------------------------------------------
# This Makefile is used for building your AVS application.
#
# It contains basic targets for building the application, installing dependencies,
# and building a Docker container.
#
# Modify each target as needed to suit your application's requirements.
# -----------------------------------------------------------------------------

GO = $(shell which go)

ROOT_DIR := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
OUT := $(ROOT_DIR)/bin
DOCKERFILE_PATH = $(ROOT_DIR)/4mica-core/Dockerfile
DOCKER_CONTEXT = $(ROOT_DIR)/4mica-core
IMAGE_NAME := ghcr.io/4mica-network/4mica-core
VERSION := latest

build:
	cd 4mica-core && cargo build --release -p core-service
	cp 4mica-core/target/release/core-service $(OUT)/performer

deps:
	GOPRIVATE=github.com/Layr-Labs/* go mod tidy

build/container:
	./.hourglass/scripts/buildContainer.sh --dockerfile $(DOCKERFILE_PATH) --context $(DOCKER_CONTEXT) --image ${IMAGE_NAME}


test: test-forge

test-forge:
	cd .devkit/contracts && forge test
