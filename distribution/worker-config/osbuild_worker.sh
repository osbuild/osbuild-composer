#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Configuring worker service"

mkdir -p /etc/osbuild-worker
tee /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
base_path = "/api/image-builder-worker/v1"

[composer]
url = ${COMPOSER_HOST}:${COMPOSER_PORT}
EOF
