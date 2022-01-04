#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Subscribing instance to RHN."

# Register the instance with RHN.
# TODO: don't store the command in a secret, only the key/org-id
/usr/local/bin/aws secretsmanager get-secret-value \
  --endpoint-url "${SECRETS_MANAGER_ENDPOINT_URL}" \
  --secret-id "${SUBSCRIPTION_MANAGER_COMMAND_ARN}" | jq -r ".SecretString" > /tmp/subscription_manager_command.json
jq -r ".subscription_manager_command" /tmp/subscription_manager_command.json | bash
rm -f /tmp/subscription_manager_command.json

subscription-manager attach --auto
