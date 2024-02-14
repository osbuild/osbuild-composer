#!/bin/bash
set -euo pipefail

# Don't subscribe on fedora
source /etc/os-release
if [ "$ID" != fedora ]; then
  /usr/local/bin/aws secretsmanager get-secret-value \
    --secret-id executor-subscription-manager-command | jq -r ".SecretString" > /tmp/subscription_manager_command.json
  jq -r ".subscription_manager_command" /tmp/subscription_manager_command.json | bash
  rm -f /tmp/subscription_manager_command.json
fi

echo "Starting osbuild-jobsite-builder."
mkdir -p /var/cache/osbuild-builder
/usr/libexec/osbuild-composer/osbuild-jobsite-builder -builder-host 0.0.0.0 -build-path /var/cache/osbuild-builder
