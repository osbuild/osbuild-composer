#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Configuring worker service"

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[composer]
url = ${COMPOSER_HOST}:${COMPOSER_PORT}
EOF
