#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Writing vector config."

ID_DOC=$(curl -Ls http://169.254.169.254/latest/dynamic/instance-identity/document)
REGION=$(echo "$ID_DOC" | jq -r .region)
PRIVATE_IP=$(echo "$ID_DOC" | jq -r .privateIp)
OSBUILD_EXECUTOR_CLOUDWATCH_GROUP=${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP:-osbuild-executor-log-group}

sudo mkdir -p /etc/vector
sudo tee /etc/vector/vector.yaml > /dev/null << EOF
sources:
  journald:
    type: journald
    exclude_units:
      - vector.service
  executor:
    type: vector
    address: ${PRIVATE_IP}:12005
sinks:
  worker_out:
    type: aws_cloudwatch_logs
    inputs:
      - journald
    region: ${REGION}
    endpoint: ${CLOUDWATCH_LOGS_ENDPOINT_URL}
    group_name: ${CLOUDWATCH_LOG_GROUP}
    stream_name: worker_syslog_{{ host }}
    encoding:
      codec: json
  executor_out:
    type: aws_cloudwatch_logs
    inputs:
      - executor
    region: ${REGION}
    endpoint: ${CLOUDWATCH_LOGS_ENDPOINT_URL}
    group_name: ${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP}
    stream_name: executor_syslog__{{ host }}
    encoding:
      codec: json
EOF

sudo systemctl enable --now vector
