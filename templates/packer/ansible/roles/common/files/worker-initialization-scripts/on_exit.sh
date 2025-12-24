#!/bin/bash
set -euo pipefail

if [ "$SERVICE_RESULT" == "success" ]; then
    exit 0
fi

echo "Worker initialization failed, setting instance state to unhealthy after grace period"
# grace period is 300 seconds, before that aws will not allow setting the instance status to unhealthy
sleep 300s
INSTANCE_ID=$(curl -Ls http://169.254.169.254/latest/meta-data/instance-id)
/usr/local/bin/aws autoscaling set-instance-health --instance-id "$INSTANCE_ID" --health-status Unhealthy
