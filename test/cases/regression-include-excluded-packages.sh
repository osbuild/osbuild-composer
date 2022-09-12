#!/bin/bash

# This test case verifies that a blueprint can include a package which
# is listed among "excluded" for a certain image type and osbuild-composer
# doesn't fail to depsolve this blueprint.
#
# The script currently works only for RHEL and CentOS which provide
# "nss-devel" package and exclude "nss" package in the image type
# definition. The testing blueprint contains explicit "nss" requirement
# to remove it from the list of excluded packages and thus enable the
# installation of "nss-devel".
#
# Bug report: https://github.com/osbuild/osbuild-composer/issues/921

set -xeuo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none
BLUEPRINT_FILE=/tmp/blueprint.toml
COMPOSE_START=/tmp/compose-start.json
COMPOSE_INFO=/tmp/compose-info.json

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "nss-devel"
description = "A base system with nss-devel"
version = "0.0.1"

[[packages]]
name = "nss-devel"

[[packages]]
# The nss package is excluded in the RHEL8.4, RHEL8.5 and RHEL9.0 qcow image type
# but it is required by the nss-devel package.This test verifies it can be added
# again when explicitly mentioned in the blueprint.
name = "nss"
EOF

sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve nss-devel
sudo composer-cli --json compose start nss-devel qcow2 | tee "${COMPOSE_START}"
COMPOSE_ID=$(get_build_info '.build_id' "$COMPOSE_START")

# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info '.queue_status' "$COMPOSE_INFO")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

sudo composer-cli compose delete "${COMPOSE_ID}" >/dev/null

jq . "${COMPOSE_INFO}"

# Did the compose finish with success?
if [[ $COMPOSE_STATUS == FINISHED ]]; then
    echo "Test passed!"
    exit 0
else
    echo "Something went wrong with the compose. 😢"
    exit 1
fi
