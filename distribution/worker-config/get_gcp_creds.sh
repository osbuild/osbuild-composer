#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Deploy GCP credentials."

if [[ -z "$GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN not defined, skipping."
  exit 0
fi

# Deploy the GCP Service Account credentials file.
aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /etc/osbuild-worker/gcp_credentials.json


tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[gcp]
credentials = "/etc/osbuild-worker/gcp_credentials.json"
EOF
