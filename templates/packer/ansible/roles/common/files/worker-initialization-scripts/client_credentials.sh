#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Writing client credentials."

if [[ -z "$CLIENT_CREDENTIALS_ARN" ]]; then
  echo "CLIENT_CREDENTIALS_ARN not defined, skipping."
  exit 0
fi

# get client credentials
/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${CLIENT_CREDENTIALS_ARN}" | jq -r ".SecretString" > /tmp/client-credentials.json

CLIENT_ID=$(jq -r ".client_id" /tmp/client-credentials.json)
jq -r ".client_secret" /tmp/client-credentials.json > /etc/osbuild-worker/client-secret
rm -f /tmp/client-credentials.json

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[authentication]
oauth_url = "${TOKEN_URL:-https://identity.api.openshift.com/auth/realms/rhoas/protocol/openid-connect/token}"
client_id = "${CLIENT_ID}"
client_secret = "/etc/osbuild-worker/client-secret"
EOF
