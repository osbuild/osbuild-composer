#!/bin/bash
set -eu

if [ $UID != 0 ]; then
  echo must be run as root
  exit 1
fi

clean_up () {
    EXIT_CODE=$?

    echo "Shutting down containers, please wait..."

    podman stop org.osbuild.koji.koji || true
    podman rm org.osbuild.koji.koji || true

    podman stop org.osbuild.koji.postgres || true
    podman rm org.osbuild.koji.postgres || true

    podman network rm -f org.osbuild.koji || true

    exit $EXIT_CODE
}

trap clean_up EXIT

podman network create org.osbuild.koji
podman run -d --name org.osbuild.koji.postgres --network org.osbuild.koji \
  -e POSTGRES_USER=koji \
  -e POSTGRES_PASSWORD=kojipass \
  -e POSTGRES_DB=koji \
  docker.io/library/postgres:12-alpine

podman run -d --name org.osbuild.koji.koji --network org.osbuild.koji \
  -p 8080:80 \
  -e POSTGRES_USER=koji \
  -e POSTGRES_PASSWORD=kojipass \
  -e POSTGRES_DB=koji \
  -e POSTGRES_HOST=org.osbuild.koji.postgres \
  quay.io/osbuild/ghci-koji:rc1

echo "Running, press CTRL+C to stop..."
sleep infinity
