#!/bin/bash
set -eu

koji_stop () {
  echo "Shutting down containers, please wait..."

  ${CONTAINER_RUNTIME} stop org.osbuild.koji.koji || true
  ${CONTAINER_RUNTIME} rm org.osbuild.koji.koji || true

  ${CONTAINER_RUNTIME} stop org.osbuild.koji.postgres || true
  ${CONTAINER_RUNTIME} rm org.osbuild.koji.postgres || true

  ${CONTAINER_RUNTIME} network rm -f org.osbuild.koji || true
}

koji_clean_up_bad_start ()  {
  EXIT_CODE=$?
  echo "Start failed, removing containers."

  koji_stop

  exit $EXIT_CODE
}

koji_start() {
  trap koji_clean_up_bad_start EXIT

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

  echo "Containers are running, to stop them use:"
  echo "$0 stop"

  trap - EXIT
}

if [[ $# -ne 1 || ( "$1" != "start" && "$1" != "stop" ) ]]; then
  cat <<DOC
usage: $0 start|stop

start - starts the koji containers
stop  - stops and removes the koji containers
DOC
  exit 3
fi

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

if [ $1 == "start" ]; then
  koji_start
fi

if [ $1 == "stop" ]; then
  koji_stop
fi
