#!/usr/bin/bash

#
# Shared helper for tests that need a PostgreSQL-backed osbuild-composer.
# Provides DB lifecycle functions (setup_db, teardown_db).
#
# API helpers (sendCompose, waitForState, collectMetrics, write_tls_composer_config)
# live in test/cases/api/common/common.sh.
#
# Usage:
#   source /usr/libexec/tests/osbuild-composer/api/common/composer-db.sh
#   setup_db
#   ...
#   # teardown_db is typically called from a trap
#

if type -p podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
  CONTAINER_RUNTIME=docker
else
  echo No container runtime found, install podman or docker.
  exit 2
fi

DB_CONTAINER_NAME="osbuild-composer-db"

function setup_db() {
    sudo "${CONTAINER_RUNTIME}" run -d --name "${DB_CONTAINER_NAME}" \
        --health-cmd "pg_isready -U postgres -d osbuildcomposer" --health-interval 2s \
        --health-timeout 2s --health-retries 10 \
        -e POSTGRES_USER=postgres \
        -e POSTGRES_PASSWORD=foobar \
        -e POSTGRES_DB=osbuildcomposer \
        -p 5432:5432 \
        --net host \
        quay.io/osbuild/postgres:13-alpine

    sudo "${CONTAINER_RUNTIME}" logs "${DB_CONTAINER_NAME}"

    pushd "$(mktemp -d)" || return 1
    sudo dnf install -y go
    go mod init temp
    go install github.com/jackc/tern@latest
    PGUSER=postgres PGPASSWORD=foobar PGDATABASE=osbuildcomposer PGHOST=localhost PGPORT=5432 \
        "$(go env GOPATH)"/bin/tern migrate -m /usr/share/tests/osbuild-composer/schemas
    popd || return 1
}

function teardown_db() {
    sudo "${CONTAINER_RUNTIME}" kill "${DB_CONTAINER_NAME}" || true
    sudo "${CONTAINER_RUNTIME}" rm "${DB_CONTAINER_NAME}" || true
}
