#!/bin/bash
set -euo pipefail

source /tmp/cloud_init_vars

echo "Writing osbuild_executor config to worker configuration."
OSBUILD_EXECUTOR_IAM_PROFILE=${OSBUILD_EXECUTOR_IAM_PROFILE:-osbuild-executor}
sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF
[osbuild_executor]
type = "aws.ec2"
iam_profile = "${OSBUILD_EXECUTOR_IAM_PROFILE}"
EOF
