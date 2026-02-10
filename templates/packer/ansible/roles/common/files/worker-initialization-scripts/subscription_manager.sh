#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Subscribing instance to RHN."

SUBSCRIPTION_MANAGER_COMMAND_ARN=${SUBSCRIPTION_MANAGER_COMMAND_ARN:-}
if [[ -z "$SUBSCRIPTION_MANAGER_COMMAND_ARN" ]]; then
  echo "SUBSCRIPTION_MANAGER_COMMAND_ARN not defined, skipping."
  exit 0
fi

# Register the instance with RHN.
# TODO: don't store the command in a secret, only the key/org-id
/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${SUBSCRIPTION_MANAGER_COMMAND_ARN}" | jq -r ".SecretString" > /tmp/subscription_manager_command.json
jq -r ".subscription_manager_command" /tmp/subscription_manager_command.json | bash
rm -f /tmp/subscription_manager_command.json
