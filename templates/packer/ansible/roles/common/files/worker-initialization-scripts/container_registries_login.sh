#!/bin/bash

set -euo pipefail
source /tmp/cloud_init_vars

echo "Login to container registries."

CONTAINER_REGISTRIES_LOGIN_ARN=${CONTAINER_REGISTRIES_LOGIN_ARN:-}
if [[ -z "$CONTAINER_REGISTRIES_LOGIN_ARN" ]]; then
  echo "CONTAINER_REGISTRIES_LOGIN_ARN not defined, skipping."
  exit 0
fi

/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${CONTAINER_REGISTRIES_LOGIN_ARN}" | jq -r ".SecretString" > /tmp/container_registries_login.json
trap "rm -f /tmp/container_registries_login.json" EXIT

for key in $(jq -r 'keys[]' /tmp/container_registries_login.json); do
    USER=$(jq -r .[\""$key"\"].username /tmp/container_registries_login.json)

    echo "Logging in to container registry $key (username: $USER)."

    PASSWORD=$(jq -r .[\""$key"\"].password /tmp/container_registries_login.json)
    echo "$PASSWORD" | sudo podman login --username="$USER" --password-stdin --authfile /etc/osbuild-worker/container-registries-auth.json "$key"
done

sudo mkdir -p /etc/systemd/system/osbuild-remote-worker@.service.d
sudo tee "/etc/systemd/system/osbuild-remote-worker@.service.d/registry-auth.conf" <<EOF
[Service]
Environment="REGISTRY_AUTH_FILE=/etc/osbuild-worker/container-registries-auth.json"
EOF
sudo systemctl daemon-reload
