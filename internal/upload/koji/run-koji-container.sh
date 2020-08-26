#!/bin/bash
set -eu

if [ $UID != 0 ]; then
  echo This script must be run as root.
  exit 1
fi

if which podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
  CONTAINER_RUNTIME=docker
else
  echo No container runtime found, install podman or docker.
  exit 2
fi

clean_up () {
    EXIT_CODE=$?

    echo "Shutting down containers, please wait..."

    ${CONTAINER_RUNTIME} stop org.osbuild.koji.koji || true
    ${CONTAINER_RUNTIME} rm org.osbuild.koji.koji || true

    ${CONTAINER_RUNTIME} stop org.osbuild.koji.postgres || true
    ${CONTAINER_RUNTIME} rm org.osbuild.koji.postgres || true

    ${CONTAINER_RUNTIME} network rm -f org.osbuild.koji || true

    exit $EXIT_CODE
}

trap clean_up EXIT

${CONTAINER_RUNTIME} network create org.osbuild.koji
${CONTAINER_RUNTIME} run -d --name org.osbuild.koji.postgres --network org.osbuild.koji \
  -e POSTGRES_USER=koji \
  -e POSTGRES_PASSWORD=kojipass \
  -e POSTGRES_DB=koji \
  docker.io/library/postgres:12-alpine

${CONTAINER_RUNTIME} run -d --name org.osbuild.koji.koji --network org.osbuild.koji \
  -p 8080:80 \
  -e POSTGRES_USER=koji \
  -e POSTGRES_PASSWORD=kojipass \
  -e POSTGRES_DB=koji \
  -e POSTGRES_HOST=org.osbuild.koji.postgres \
  quay.io/osbuild/ghci-koji:v1

echo "Running, press CTRL+C to stop..."
sleep infinity
