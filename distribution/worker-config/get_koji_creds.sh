#!/bin/bash
set -eo pipefail
source /tmp/cloud_init_vars

echo "Deploy Koji credentials."

if [[ -z "$KOJI_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "KOJI_ACCOUNT_IMAGE_BUILDER_ARN not defined, skipping."
  exit 0
fi

aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${KOJI_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /tmp/koji_credentials.json

KOJIHUB=$(jq -r ".kojihub" /tmp/koji_credentials.json)
PRINCIPAL=$(jq -r ".principal" /tmp/koji_credentials.json)

jq -r ".keytab" /tmp/koji_credentials.json | base64 -d >/etc/osbuild-worker/koji.keytab
rm /tmp/koji_credentials.json

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[koji."${KOJIHUB}".kerberos]
principal = "${PRINCIPAL}"
keytab = "/etc/osbuild-worker/koji.keytab"
EOF

