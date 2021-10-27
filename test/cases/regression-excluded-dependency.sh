#!/bin/bash

# This test case verifies that a blueprint can include a package which has a
# dependency that is listed among "excluded" for a certain image type and
# osbuild-composer doesn't fail to depsolve this blueprint.
#
# The script currently works only for RHEL and CentOS which provide
# "nss-devel" package and exclude "nss" package in the image type
# definition. The testing contains the "nss-devel" package which can only
# be installed if the "nss" package isn't excluded
#
# Bug report: https://github.com/osbuild/osbuild-composer/issues/921

# Get OS data.
source /etc/os-release

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

case "${ID}-${VERSION_ID}" in
    "rhel-8.6" | "rhel-9.0" | "centos-9")
        ;;
    *)
        echo "$0 is not enabled for ${ID}-${VERSION_ID} skipping..."
        exit 0
        ;;
esac


set -xeuo pipefail

# Provision the software under test.
BLUEPRINT_FILE=/tmp/blueprint.toml
COMPOSE_START=/tmp/compose-start.json
COMPOSE_INFO=/tmp/compose-info.json

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "nss-devel"
description = "A base system with nss-devel"
version = "0.0.1"

# The nss package is excluded in the RHEL 8.5 and RHEL 9.0 qcow image type
# and is required by the nss-devel package. This test verifies the excluded
# dependency doesn't restrict the installation of the dependant.
[[packages]]
name = "nss-devel"
EOF

sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve nss-devel
sudo composer-cli --json compose start nss-devel qcow2 | tee "${COMPOSE_START}"
if rpm -q --quiet weldr-client; then
    COMPOSE_ID=$(jq -r '.body.build_id' "$COMPOSE_START")
else
    COMPOSE_ID=$(jq -r '.build_id' "$COMPOSE_START")
fi
# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    if rpm -q --quiet weldr-client; then
        COMPOSE_STATUS=$(jq -r '.body.queue_status' "$COMPOSE_INFO")
    else
        COMPOSE_STATUS=$(jq -r '.queue_status' "$COMPOSE_INFO")
    fi

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
