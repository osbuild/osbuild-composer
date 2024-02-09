#!/bin/bash
set -euo pipefail

/usr/local/bin/aws secretsmanager get-secret-value \
  --secret-id executor-subscription-manager-command | jq -r ".SecretString" > /tmp/subscription_manager_command.json
jq -r ".subscription_manager_command" /tmp/subscription_manager_command.json | bash
rm -f /tmp/subscription_manager_command.json

echo "Starting osbuild-jobsite-builder."
/usr/libexec/osbuild-composer/osbuild-jobsite-builder
