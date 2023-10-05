#!/bin/bash
# AppSRE runs this script to build the container and push it to Quay.
set -exv

IMAGE_NAME="quay.io/app-sre/composer"
IMAGE_TAG=$(git rev-parse --short=7 HEAD)
COMMIT=$(git rev-parse HEAD)

if [[ -z "$QUAY_USER" || -z "$QUAY_TOKEN" ]]; then
    echo "QUAY_USER and QUAY_TOKEN must be set"
    exit 1
fi

DOCKER_CONF="$PWD/.docker"
mkdir -p "$DOCKER_CONF"
docker --config="$DOCKER_CONF" login -u="$QUAY_USER" -p="$QUAY_TOKEN" quay.io
docker --config="$DOCKER_CONF" build -f distribution/Dockerfile-ubi --build-arg="COMMIT=${COMMIT}" -t "${IMAGE_NAME}:${IMAGE_TAG}" .
docker --config="$DOCKER_CONF" push "${IMAGE_NAME}:${IMAGE_TAG}"

# Maintenance image
IMAGE_NAME="quay.io/app-sre/composer-maintenance"
IMAGE_TAG=$(git rev-parse --short=7 HEAD)
docker --config="$DOCKER_CONF" build -f distribution/Dockerfile-ubi-maintenance -t "${IMAGE_NAME}:${IMAGE_TAG}" .
docker --config="$DOCKER_CONF" push "${IMAGE_NAME}:${IMAGE_TAG}"
