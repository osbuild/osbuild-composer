#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Deploy AWS credentials."


echo "Write the bucket."
# Always create the header and write the bucket, it's slightly ugly but it will work
sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[aws]
bucket = "${WORKER_CONFIG_AWS_BUCKET:-}"
EOF

if [[ -z "$AWS_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "AWS_ACCOUNT_IMAGE_BUILDER_ARN not defined, skipping."
  exit 0
fi

aws secretsmanager get-secret-value \
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

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
credentials = "${WORKER_CONFIG_AWS_CREDENTIALS:-}"
EOF
