#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Deploy AWS credentials."

# Deploy the AWS credentials file if the secret ARN was set.
if [[ -n "$AWS_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  /usr/local/bin/aws secretsmanager get-secret-value \
    --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
    --secret-id "${AWS_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /tmp/aws_credentials.json
  ACCESS_KEY_ID=$(jq -r ".access_key_id" /tmp/aws_credentials.json)
  SECRET_ACCESS_KEY=$(jq -r ".secret_access_key" /tmp/aws_credentials.json)
  rm /tmp/aws_credentials.json

  sudo tee /etc/osbuild-worker/aws_credentials.toml > /dev/null << EOF
[default]
aws_access_key_id = "$ACCESS_KEY_ID"
aws_secret_access_key = "$SECRET_ACCESS_KEY"
EOF

fi
