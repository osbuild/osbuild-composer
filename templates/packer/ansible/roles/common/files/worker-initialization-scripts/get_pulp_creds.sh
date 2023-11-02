#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Deploy Pulp credentials."

if [[ -z "$PULP_PASSWORD_ARN" ]]; then
  echo "PULP_PASSWORD_ARN not defined, skipping."
  exit 0
fi

/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${PULP_PASSWORD_ARN}" | jq -r ".SecretString" > /tmp/pulp_credentials.json

PULP_PASSWORD=$(jq -r ".password" /tmp/pulp_credentials.json)
rm /tmp/pulp_credentials.json

PULP_USERNAME=${PULP_USERNAME:-admin}
PULP_SERVER=${PULP_SERVER:-}

sudo tee /etc/osbuild-worker/pulp_credentials.json > /dev/null << EOF
{
  "username": "$PULP_USERNAME",
  "password": "$PULP_PASSWORD"
}
EOF

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[pulp]
server_address = "$PULP_SERVER"
credentials = "/etc/osbuild-worker/pulp_credentials.json"
EOF
