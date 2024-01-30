#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Deploy GCP credentials."

echo "Write the bucket."
# Always create the header and write the bucket, it's slightly ugly but it will work
# The bucket is always set, becuase the instance can potentially authenticate to GCP
# with a service account connected to it, without any explicit credentials.
sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[gcp]
bucket = "${WORKER_CONFIG_GCP_BUCKET:-}"
EOF

GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN=${GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN:-}
if [[ -z "$GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN not defined, skipping."
  exit 0
fi

# Deploy the GCP Service Account credentials file.
/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /etc/osbuild-worker/gcp_credentials.json


sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
credentials = "/etc/osbuild-worker/gcp_credentials.json"
EOF
