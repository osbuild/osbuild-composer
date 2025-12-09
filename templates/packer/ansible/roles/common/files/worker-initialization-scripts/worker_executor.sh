#!/bin/bash
set -euo pipefail

source /etc/os-release
# /tmp/cloud_init_vars may not exist on the osbuild-executor
source /tmp/cloud_init_vars || true

echo "Writing vector config."
HOST_WORKER_ADDRESS=${HOST_WORKER_ADDRESS:-127.0.0.1}

if [ "$HOST_WORKER_ADDRESS" != "127.0.0.1" ]; then
    sudo mkdir -p /etc/vector
    sudo tee /etc/vector/vector.yaml > /dev/null << EOF
sources:
  journald:
    type: journald
    exclude_units:
      - vector.service
sinks:
  host_worker:
    type: vector
    inputs:
      - journald
    address: ${HOST_WORKER_ADDRESS}:12005
EOF
    # in some cases vector might need a few restarts if the network
    # isn't fully initialized yet (specifically on the internal network).
    set +e
    for i in $(seq 1 8); do
        if sudo systemctl enable --now vector; then
            break
        fi
        echo "vector failed to start, backing off and retrying (attempt $i)"
        sleep 5s
    done
    set -e
fi

echo "Starting worker executor"
/usr/libexec/osbuild-composer/osbuild-worker-executor -host 0.0.0.0
