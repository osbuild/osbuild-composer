#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Writing offline token."

# get offline token
/usr/local/bin/aws secretsmanager get-secret-value \
    --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
    --secret-id "${OFFLINE_TOKEN_ARN}" | jq -r ".SecretString" >/tmp/offline-token.json

mkdir /etc/osbuild-worker
jq -r ".offline_token" /tmp/offline-token.json >/etc/osbuild-worker/offline-token
rm -f /tmp/offline-token.json
