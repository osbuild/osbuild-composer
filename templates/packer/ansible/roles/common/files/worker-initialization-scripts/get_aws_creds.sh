#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Deploy AWS credentials."


echo "Write the bucket."
# Always create the header and write the bucket, it's slightly ugly but it will work
# The bucket is always set, becuase the instance can potentially authenticate to AWS
# with its instance profile, without any explicit credentials.
sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[aws]
bucket = "${WORKER_CONFIG_AWS_BUCKET:-}"
EOF

AWS_ACCOUNT_IMAGE_BUILDER_ARN=${AWS_ACCOUNT_IMAGE_BUILDER_ARN:-}
if [[ -n "$AWS_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "AWS_ACCOUNT_IMAGE_BUILDER_ARN set, configuring aws credentials"

  /usr/local/bin/aws secretsmanager get-secret-value \
    --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
    --secret-id "${AWS_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /tmp/aws_credentials.json
  ACCESS_KEY_ID=$(jq -r ".access_key_id" /tmp/aws_credentials.json)
  SECRET_ACCESS_KEY=$(jq -r ".secret_access_key" /tmp/aws_credentials.json)
  rm /tmp/aws_credentials.json

  CREDS_FILE="/etc/osbuild-worker/aws_credentials.toml"
  sudo tee "$CREDS_FILE" > /dev/null << EOF
[default]
aws_access_key_id = "$ACCESS_KEY_ID"
aws_secret_access_key = "$SECRET_ACCESS_KEY"
EOF
  sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
credentials = "$CREDS_FILE"
EOF
fi

AWS_S3_ACCOUNT_IMAGE_BUILDER_ARN=${AWS_S3_ACCOUNT_IMAGE_BUILDER_ARN:-}
if [[ -n "$AWS_S3_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "AWS_S3_ACCOUNT_IMAGE_BUILDER_ARN set, configuring aws credentials"

  /usr/local/bin/aws secretsmanager get-secret-value \
    --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
    --secret-id "${AWS_S3_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /tmp/aws_credentials.json
  ACCESS_KEY_ID=$(jq -r ".access_key_id" /tmp/aws_credentials.json)
  SECRET_ACCESS_KEY=$(jq -r ".secret_access_key" /tmp/aws_credentials.json)
  rm /tmp/aws_credentials.json

  CREDS_FILE="/etc/osbuild-worker/aws_s3_credentials.toml"
  sudo tee "$CREDS_FILE" > /dev/null << EOF
[default]
aws_access_key_id = "$ACCESS_KEY_ID"
aws_secret_access_key = "$SECRET_ACCESS_KEY"
EOF
  sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
s3_credentials = "$CREDS_FILE"
EOF
fi
