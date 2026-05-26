#!/bin/bash

#
# Test that osbuild failures end up in the journal and log.
#

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

TEMPDIR=$(mktemp -d)
BP=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start.json
COMPOSE_INFO=${TEMPDIR}/compose-info.json

tee "$BP" > /dev/null << EOF
name = "bp"
description = "A base system"
version = "0.0.1"

[customizations]
[customizations.services]
enabled = ["blergh"]
EOF

sudo composer-cli blueprints push "$BP"

sudo composer-cli --json compose start bp wsl | tee "$COMPOSE_START"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

JOURNAL_OUTPUT=${TEMPDIR}/journal
sudo journalctl -u osbuild-worker@1 --output cat | sudo tee "$JOURNAL_OUTPUT"
LOG_OUTPUT=${TEMPDIR}/log
sudo composer-cli compose log "$COMPOSE_ID" | sudo tee "$LOG_OUTPUT"

EXPECTED_OUTPUT=(
    "Failed to enable unit: Unit blergh.service does not exist"
    "Traceback (most recent call last):"
    "CalledProcessError.*systemctl.*enable.*blergh.*returned non-zero exit status 1."
)
for i in "${!EXPECTED_OUTPUT[@]}"; do
    if ! grep -q "${EXPECTED_OUTPUT[$i]}" "$JOURNAL_OUTPUT"; then
        redprint "\"${EXPECTED_OUTPUT[$i]}\" not found in journal"
        exit 1
    fi
    if ! grep -q "${EXPECTED_OUTPUT[$i]}" "$LOG_OUTPUT"; then
        redprint "\"${EXPECTED_OUTPUT[$i]}\" not found in compose log"
        exit 1
    fi
done

# the worker also logs the JobError which includes the entire error in one line as part of the error details
if ! grep -q "failure details.*Failed to enable unit: Unit blergh.service does not exist" "$JOURNAL_OUTPUT"; then
    redprint "failure details not found in journal"
    exit 1
fi
