#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Deploy Azure credentials."

AZURE_ACCOUNT_IMAGE_BUILDER_ARN=${AZURE_ACCOUNT_IMAGE_BUILDER_ARN:-}
if [[ -z "$AZURE_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "AZURE_ACCOUNT_IMAGE_BUILDER_ARN not defined, skipping."
  exit 0
fi

# Deploy the Azure credentials file.
/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${AZURE_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /tmp/azure_credentials.json
CLIENT_ID=$(jq -r ".client_id" /tmp/azure_credentials.json)
CLIENT_SECRET=$(jq -r ".client_secret" /tmp/azure_credentials.json)
rm /tmp/azure_credentials.json

sudo tee /etc/osbuild-worker/azure_credentials.toml > /dev/null << EOF
client_id =     "$CLIENT_ID"
client_secret = "$CLIENT_SECRET"
EOF

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[azure]
credentials = "/etc/osbuild-worker/azure_credentials.toml"
EOF
