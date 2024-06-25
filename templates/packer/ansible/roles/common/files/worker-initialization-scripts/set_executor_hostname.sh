#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

if [[ -z "$OSBUILD_EXECUTOR_HOSTNAME" ]]; then
    echo "OSBUILD_EXECUTOR_HOSTNAME not set, skipping."
    exit 0
fi

echo "Setting system hostname to $OSBUILD_EXECUTOR_HOSTNAME."
hostnamectl set-hostname "$OSBUILD_EXECUTOR_HOSTNAME"
