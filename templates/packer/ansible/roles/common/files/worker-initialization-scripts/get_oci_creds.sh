#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Deploy OCI credentials."

OCI_ACCOUNT_IMAGE_BUILDER_ARN=${OCI_ACCOUNT_IMAGE_BUILDER_ARN:-}
if [[ -z "$OCI_ACCOUNT_IMAGE_BUILDER_ARN" ]]; then
  echo "OCI_ACCOUNT_IMAGE_BUILDER_ARN not defined, skipping."
  exit 0
fi

/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${OCI_ACCOUNT_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /tmp/oci_credentials.json

USER=$(jq -r ".user" /tmp/oci_credentials.json)
TENANCY=$(jq -r ".tenancy" /tmp/oci_credentials.json)
REGION=$(jq -r ".region" /tmp/oci_credentials.json)
FINGERPRINT=$(jq -r ".fingerprint" /tmp/oci_credentials.json)
NAMESPACE=$(jq -r ".namespace" /tmp/oci_credentials.json)
BUCKET_NAME=$(jq -r ".bucket" /tmp/oci_credentials.json)
COMPARTMENT=$(jq -r ".compartment" /tmp/oci_credentials.json)
PRIV_KEY_DATA=$(jq -r ".priv_key_data" /tmp/oci_credentials.json)

rm /tmp/oci_credentials.json

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[oci]
credentials = "/etc/osbuild-worker/oci-credentials.toml"
EOF

sudo tee /etc/osbuild-worker/oci-credentials.toml > /dev/null << EOF
user = "$USER"
tenancy = "$TENANCY"
region = "$REGION"
fingerprint = "$FINGERPRINT"
namespace = "$NAMESPACE"
bucket = "$BUCKET_NAME"
compartment = "$COMPARTMENT"
private_key = """
$PRIV_KEY_DATA
"""
EOF
