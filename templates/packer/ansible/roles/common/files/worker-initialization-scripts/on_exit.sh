#!/bin/bash
set -euo pipefail

if [ "$SERVICE_RESULT" == "success" ]; then
    exit 0
fi

echo "Worker initialization failed,  setting instance state to unhealthy"
INSTANCE_ID=$(curl -Ls http://169.254.169.254/latest/meta-data/instance-id)
/usr/local/bin/aws autoscaling set-instance-health --instance-id "$INSTANCE_ID" --health-status Unhealthy
