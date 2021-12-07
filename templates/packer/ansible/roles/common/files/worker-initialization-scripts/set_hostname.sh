#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

# Get the instance ID.
INSTANCE_ID=$(curl -Ls http://169.254.169.254/latest/meta-data/instance-id)

# Assemble hostname.
FULL_HOSTNAME="${SYSTEM_HOSTNAME_PREFIX}-${INSTANCE_ID}"

# Print out the new hostname.
echo "Setting system hostname to ${FULL_HOSTNAME}."

# Set the system hostname.
hostnamectl set-hostname "$FULL_HOSTNAME"
