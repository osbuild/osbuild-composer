#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Writing offline token."

if [[ -z "$OFFLINE_TOKEN_ARN" ]]; then
  echo "OFFLINE_TOKEN_ARN not defined, skipping."
  exit 0
fi

# get offline token
/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${OFFLINE_TOKEN_ARN}" | jq -r ".SecretString" > /tmp/offline-token.json

jq -r ".offline_token" /tmp/offline-token.json > /etc/osbuild-worker/offline-token
rm -f /tmp/offline-token.json

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[authentication]
oauth_url = "${TOKEN_URL:-https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/token}"
client_id = "rhsm-api"
offline_token = "/etc/osbuild-worker/offline-token"
EOF
