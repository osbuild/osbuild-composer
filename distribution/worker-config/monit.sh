#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Writing monit config."

cp /usr/share/osbuild-worker-config/monitrc /etc/monitrc