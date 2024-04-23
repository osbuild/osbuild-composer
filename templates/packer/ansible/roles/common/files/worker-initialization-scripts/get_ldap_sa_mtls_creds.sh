#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Deploy MTLS credentials for custom repositories."

LDAP_SERVICE_ACCOUNT_MTLS_IMAGE_BUILDER_ARN=${LDAP_SERVICE_ACCOUNT_MTLS_IMAGE_BUILDER_ARN:-}
if [[ -z "$LDAP_SERVICE_ACCOUNT_MTLS_IMAGE_BUILDER_ARN" ]]; then
  echo "LDAP_SERVICE_ACCOUNT_MTLS_IMAGE_BUILDER_ARN not defined, skipping."
  exit 0
fi

/usr/local/bin/aws secretsmanager get-secret-value \
--endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
--secret-id "${LDAP_SERVICE_ACCOUNT_MTLS_IMAGE_BUILDER_ARN}" | jq -r ".SecretString" > /tmp/ldap_service_account_mtls_credentials.json
MTLS_CERT=$(jq -r ".cert" /tmp/ldap_service_account_mtls_credentials.json)
MTLS_KEY=$(jq -r ".key" /tmp/ldap_service_account_mtls_credentials.json)
BASEURL=$(jq -r ".baseurl" /tmp/ldap_service_account_mtls_credentials.json)
CA=$(jq -r ".ca" /tmp/ldap_service_account_mtls_credentials.json)
PROXY=$(jq -r ".proxy" /tmp/ldap_service_account_mtls_credentials.json)
rm /tmp/ldap_service_account_mtls_credentials.json

sudo tee /etc/osbuild-worker/image_builder_sa_mtls_cert.pem > /dev/null << EOF
$MTLS_CERT
EOF

sudo tee /etc/osbuild-worker/image_builder_sa_mtls_key.pem > /dev/null << EOF
$MTLS_KEY
EOF

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[repository_mtls]
baseurl = "$BASEURL"
mtls_client_key = "/etc/osbuild-worker/image_builder_sa_mtls_key.pem"
mtls_client_cert = "/etc/osbuild-worker/image_builder_sa_mtls_cert.pem"
EOF

if [ "$PROXY" != null ]; then
    sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
proxy = "$PROXY"
EOF
fi

if [ "$CA" != null ]; then
    sudo tee /etc/osbuild-worker/image_builder_sa_ca.pem > /dev/null << EOF
$CA
EOF
    sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
ca = "/etc/osbuild-worker/image_builder_sa_ca.pem"
EOF
fi
