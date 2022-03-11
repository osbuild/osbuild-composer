#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Deploy GCP credentials."

# Deploy the GCP Service Account credentials file.
/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${GCP_SERVICE_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /etc/osbuild-worker/gcp_credentials.json
