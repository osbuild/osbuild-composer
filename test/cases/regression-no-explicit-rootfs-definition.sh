#!/bin/bash
# https://bugzilla.redhat.com/show_bug.cgi?id=2049500


# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

case "${ID}-${VERSION_ID}" in
    "fedora-"*)
        echo "$0 is not enabled for ${ID}-${VERSION_ID} skipping..."
        exit 0
        ;;
    *)
        ;;
esac

set -xeuo pipefail

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none


function get_build_info() {
    key="$1"
    fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
    fi
    jq -r "${key}" "${fname}"
}

# Provision the software under test.
BLUEPRINT_FILE=/tmp/blueprint.toml
COMPOSE_START=/tmp/compose-start.json
COMPOSE_INFO=/tmp/compose-info.json

# Write a basic blueprint for our image.
# It is important that / is not explicitly specified in the blueprint
tee "$BLUEPRINT_FILE" > /dev/null << 'EOF'
name = "noexplicitroot"
description = "no explicit rootfs defined"
version = "0.0.1"
modules = []
groups = []

[[packages]]
name = "vim-enhanced"
version = "*"

[[customizations.filesystem]]
mountpoint = "/data"
size = 2147483648
EOF

sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli --json compose start noexplicitroot qcow2 | tee "${COMPOSE_START}"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")
# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
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
