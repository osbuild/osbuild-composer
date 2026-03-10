#!/bin/bash

#
# Test the worker's clean_store config option.
#

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

set -euo pipefail

/usr/libexec/osbuild-composer-test/provision.sh none

function cleanup() {
    greenprint cleanup
    set +eu

    sudo journalctl -u osbuild-worker@1.service
}
trap cleanup EXIT

# enable clean_store & restart worker
cat <<EOF | sudo tee /etc/osbuild-worker/osbuild-worker2.toml
clean_store = true
$(cat /etc/osbuild-worker/osbuild-worker.toml)
EOF
sudo mv -fv /etc/osbuild-worker/osbuild-worker2.toml /etc/osbuild-worker/osbuild-worker.toml
sudo systemctl restart osbuild-worker@1.service


tee "bp.toml" > /dev/null << EOF
name = "basic"
description = "empty"
version = "0.0.1"
EOF
sudo composer-cli blueprints push bp.toml

TEMPDIR=$(mktemp -d)
COMPOSE_START=${TEMPDIR}/compose-start.json
COMPOSE_INFO=${TEMPDIR}/compose-info.json
sudo composer-cli --json compose start basic wsl | tee "$COMPOSE_START"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")
greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")
    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi
    # Wait 5 seconds and try again.
    sleep 5
done


# make sure the store is empty
if [ -d /var/cache/osbuild-worker/osbuild-store ]; then
    redprint "/var/cache/osbuild-worker/osbuild-store still exists"
    exit 1
fi

# negative scenario: if cleaning out the store fails, ensure the worker
# is stopped
sudo mkdir /var/cache/osbuild-worker/osbuild-store
sudo chattr +a /var/cache/osbuild-worker/osbuild-store
sudo composer-cli --json compose start basic wsl | tee "$COMPOSE_START"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")
greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")
    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi
    # Wait 5 seconds and try again.
    sleep 5
done
sudo chattr -a /var/cache/osbuild-worker/osbuild-store

if sudo systemctl is-active osbuild-worker@1.service; then
   redprint "expected worker to stop itself after failing to clean out store"
   exit 1
fi

greenprint "clean_store works as expected"
exit 0
