#!/bin/bash

# This test case verifies that a blueprint can include a package which has a
# dependency that is listed among "excluded" for a certain image type and
# osbuild-composer doesn't fail to depsolve this blueprint.
#
# The script currently works only for RHEL and CentOS which provide
# "redhat-lsb-core" package and exclude "nss" package in the image type
# definition. The testing contains the "redhat-lsb-core" package which can only
# be installed if the "nss" package isn't excluded
#
# Bug report: https://github.com/osbuild/osbuild-composer/issues/921

# NOTE: ONLY WORKS IN RHEL 8.5
# Get OS data.
source /etc/os-release

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

if [[ "${ID}-${VERSION_ID}" != "rhel-8.5" ]]; then
    echo "$0 is only enabled for rhel-8.5; skipping..."
    exit 0
fi

set -xeuo pipefail

# Provision the software under tet.
BLUEPRINT_FILE=/tmp/blueprint.toml
COMPOSE_START=/tmp/compose-start.json
COMPOSE_INFO=/tmp/compose-info.json

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "redhat-lsb-core"
description = "A base system with redhat-lsb-core"
version = "0.0.1"

# The nss package is excluded in the RHEL 8.5 image type and is required by the
# redhat-lsb-core package. This test verifies the excluded dependency doesn't
# restrict the installation of the dependant.
[[packages]]
name = "redhat-lsb-core"
EOF

sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve redhat-lsb-core
sudo composer-cli --json compose start redhat-lsb-core qcow2 | tee "${COMPOSE_START}"
COMPOSE_ID=$(jq -r '.build_id' "$COMPOSE_START")

# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(jq -r '.queue_status' "$COMPOSE_INFO")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

jq . "${COMPOSE_INFO}"

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi
