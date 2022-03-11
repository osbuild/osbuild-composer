#!/bin/bash
set -euo pipefail
source /tmp/cloud_init_vars

echo "Writing vector config."

sudo mkdir -p /etc/vector
sudo tee /etc/vector/vector.toml >/dev/null <<EOF
[sources.journald]
type = "journald"
exclude_units = ["vector.service"]

[sinks.out]
type = "aws_cloudwatch_logs"
inputs = [ "journald" ]
endpoint = "${CLOUDWATCH_LOGS_ENDPOINT_URL}"
group_name = "${CLOUDWATCH_LOG_GROUP}"
stream_name = "worker_syslog_{{ host }}"
encoding.codec = "json"
EOF

sudo systemctl enable --now vector
