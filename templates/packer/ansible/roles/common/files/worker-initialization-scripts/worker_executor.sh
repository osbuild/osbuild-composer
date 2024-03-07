#!/bin/bash
set -euo pipefail

source /etc/os-release
source /tmp/cloud_init_vars

# Don't subscribe on fedora
if [ "$ID" != fedora ]; then
  /usr/local/bin/aws secretsmanager get-secret-value \
    --secret-id executor-subscription-manager-command | jq -r ".SecretString" > /tmp/subscription_manager_command.json
  jq -r ".subscription_manager_command" /tmp/subscription_manager_command.json | bash
  rm -f /tmp/subscription_manager_command.json
fi

echo "Writing vector config."
REGION=$(curl -Ls http://169.254.169.254/latest/dynamic/instance-identity/document | jq -r .region)
HOSTNAME=$(hostname)
CLOUDWATCH_ENDPOINT="https://logs.$REGION.amazonaws.com"
OSBUILD_EXECUTOR_CLOUDWATCH_GROUP=${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP:-osbuild-executor-log-group}

sudo mkdir -p /etc/vector
sudo tee /etc/vector/vector.yaml > /dev/null << EOF
sources:
  journald:
    type: journald
    exclude_units:
      - vector.service
sinks:
  out:
    type: aws_cloudwatch_logs
    inputs:
      - journald
    region: ${REGION}
    endpoint: ${CLOUDWATCH_ENDPOINT}
    group_name: ${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP}
    stream_name: worker_syslog_{{ host }}
    encoding:
      codec: json
EOF
sudo systemctl enable --now vector

echo "Starting osbuild-jobsite-builder."
mkdir -p /var/cache/osbuild-builder
/usr/libexec/osbuild-composer/osbuild-jobsite-builder -builder-host 0.0.0.0 -build-path /var/cache/osbuild-builder
