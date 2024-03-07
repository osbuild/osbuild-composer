#!/bin/bash
set -euo pipefail

source /tmp/cloud_init_vars

echo "Writing osbuild_executor config to worker configuration."
OSBUILD_EXECUTOR_IAM_PROFILE=${OSBUILD_EXECUTOR_IAM_PROFILE:-osbuild-executor}
OSBUILD_EXECUTOR_CLOUDWATCH_GROUP=${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP:-}

CLOUDWATCH_GROUP_CONFIG=""
if [ -n "${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP}" ]; then
    CLOUDWATCH_GROUP_CONFIG="cloudwatch_group = \"${OSBUILD_EXECUTOR_CLOUDWATCH_GROUP}\"\n"
fi

sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[osbuild_executor]
type = "aws.ec2"
iam_profile = "${OSBUILD_EXECUTOR_IAM_PROFILE}"
${CLOUDWATCH_GROUP_CONFIG}
EOF
