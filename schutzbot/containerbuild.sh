#!/bin/bash
set -euo pipefail

echo "Prepare host system"

sudo dnf -y install podman

echo "Build container"

IMAGE_NAME="quay.io/osbuild/osbuild-composer-ubi-pr"
IMAGE_TAG="${CI_COMMIT_SHA:-$(git rev-parse HEAD)}"

podman \
	build \
	--file="distribution/Dockerfile-ubi" \
	--tag="${IMAGE_NAME}:${IMAGE_TAG}" \
	--build-arg="COMMIT=${IMAGE_TAG}" \
	--label="quay.expires-after=1w" \
	.

# Push to reuse later in the pipeline (see regression tests)
BRANCH_NAME="${BRANCH_NAME:-${CI_COMMIT_BRANCH}}"
podman push \
       --creds "${V2_QUAY_USERNAME}":"${V2_QUAY_PASSWORD}" \
       "${IMAGE_NAME}:${IMAGE_TAG}"
