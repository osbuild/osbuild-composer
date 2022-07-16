#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Starting worker service and monit."

# Prepare osbuild-composer's remote worker services and sockets.
systemctl enable --now "osbuild-remote-worker@${COMPOSER_HOST}:${COMPOSER_PORT}"

# Now that everything is configured, ensure monit is monitoring everything.
systemctl enable --now monit
