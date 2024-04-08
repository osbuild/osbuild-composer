#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Starting worker service."

http_proxy=${http_proxy:-}
https_proxy=${https_proxy:-}
if [ -n "$http_proxy" ] || [ -n "$https_proxy" ]; then
   sudo mkdir /etc/systemd/system/osbuild-remote-worker@.service.d/
   sudo tee -a /etc/systemd/system/osbuild-remote-worker@.service.d/override.conf <<EOF
[Service]
Environment="http_proxy=$http_proxy"
Environment="https_proxy=$https_proxy"
EOF
fi

# Prepare osbuild-composer's remote worker services and sockets.
systemctl enable --now "osbuild-remote-worker@${COMPOSER_HOST}:${COMPOSER_PORT}"
