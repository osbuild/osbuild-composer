#!/bin/bash
set -euo pipefail


echo "Query host"

COMMIT=$(git rev-parse HEAD)


echo "Prepare host system"

sudo dnf -y install podman


echo "Build container"

podman \
	build \
	"--file=distribution/Dockerfile-ubi" \
	"--tag=osbuild-composer:${COMMIT}" \
	.
