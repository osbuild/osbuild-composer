#!/bin/bash
set -euo pipefail

source /etc/os-release
# /tmp/cloud_init_vars may not exist on the osbuild-executor
source /tmp/cloud_init_vars || true

echo "Writing vector config."
REGION=$(curl -Ls http://169.254.169.254/latest/dynamic/instance-identity/document | jq -r .region)
HOSTNAME=$(hostname)
CLOUDWATCH_ENDPOINT="https://logs.$REGION.amazonaws.com"
OSBUILD_EXECUTOR_CLOUDWATCH_GROUP=${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP:-osbuild-executor-log-group}
HOST_WORKER_ADDRESS=${HOST_WORKER_ADDRESS:-127.0.0.1}

sudo mkdir -p /etc/vector
sudo tee /etc/vector/vector.yaml > /dev/null << EOF
sources:
  journald:
    type: journald
    exclude_units:
      - vector.service
sinks:
  cloudwatch:
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

if [ "$HOST_WORKER_ADDRESS" != "127.0.0.1" ]; then
    sudo tee -a /etc/vector/vector.yaml > /dev/null <<EOF
  host_worker:
    type: vector
    inputs:
      - journald
    address: ${HOST_WORKER_ADDRESS}:12005
EOF
fi

sudo systemctl enable --now vector

echo "Starting worker executor"
/usr/libexec/osbuild-composer/osbuild-worker-executor -host 0.0.0.0
