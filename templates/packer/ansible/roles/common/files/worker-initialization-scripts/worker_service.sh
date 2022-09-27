#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Starting worker service."

# Prepare osbuild-composer's remote worker services and sockets.
systemctl enable --now "osbuild-remote-worker@${COMPOSER_HOST}:${COMPOSER_PORT}"
