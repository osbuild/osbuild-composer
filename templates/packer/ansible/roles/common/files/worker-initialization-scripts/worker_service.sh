#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Setting up worker services."

sudo tee /etc/osbuild-worker/osbuild-worker.toml >/dev/null <<EOF
base_path = "/api/image-builder-worker/v1"
[authentication]
oauth_url = "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token"
offline_token = "/etc/osbuild-worker/offline-token"
[gcp]
credentials = "/etc/osbuild-worker/gcp_credentials.json"
[azure]
credentials = "/etc/osbuild-worker/azure_credentials.toml"
[aws]
credentials = "${WORKER_CONFIG_AWS_CREDENTIALS:-}"
bucket = "${WORKER_CONFIG_AWS_BUCKET:-}"
EOF

# Prepare osbuild-composer's remote worker services and sockets.
systemctl enable --now "osbuild-remote-worker@${COMPOSER_HOST}:${COMPOSER_PORT}"

# Now that everything is configured, ensure monit is monitoring everything.
systemctl enable --now monit
