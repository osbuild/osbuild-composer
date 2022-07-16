#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Configuring worker service"

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[composer]
url = ${COMPOSER_HOST}:${COMPOSER_PORT}
EOF

echo "Starting worker service and monit."

# Prepare osbuild-composer's remote worker services and sockets.
systemctl enable --now osbuild-worker

# Now that everything is configured, ensure monit is monitoring everything.
systemctl enable --now monit
