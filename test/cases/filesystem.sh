#!/usr/bin/bash

#
# Test the ability to specify custom mountpoints for RHEL8.5 and above
#

source /etc/os-release

if [[ "${ID}-${VERSION_ID}" != "rhel-8.5" ]]; then
    echo "$0 is only enabled for rhel-8.5; skipping..."
    exit 0
fi

set -xeuo pipefail

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

TEST_UUID=$(uuidgen)
IMAGE_KEY="osbuild-composer-test-${TEST_UUID}"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# Workaround the problem that 'image-info' can not read SELinux labels unknown to the host from the image
OSBUILD_LABEL=$(matchpathcon -n "$(which osbuild)")
sudo chcon "$OSBUILD_LABEL" /usr/libexec/osbuild-composer-test/image-info

# Build ostree image.
build_image() {
    blueprint_file=$1
    blueprint_name=$2
    image_type=$3

    # Prepare the blueprint for the compose.
    greenprint "📋 Preparing blueprint"
    sudo composer-cli blueprints push "$blueprint_file"
    sudo composer-cli blueprints depsolve "$blueprint_name"

    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!
    # Stop watching the worker journal when exiting.
    trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

    # Start the compose.
    greenprint "🚀 Starting compose"
    sudo composer-cli --json compose start "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    COMPOSE_ID=$(jq -r '.build_id' "$COMPOSE_START")

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
        COMPOSE_STATUS=$(jq -r '.queue_status' "$COMPOSE_INFO")

        # Is the compose finished?
        if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 5
    done

    # Kill the journal monitor immediately and remove the trap
    sudo pkill -P ${WORKER_JOURNAL_PID}
    trap - EXIT

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi
}

# Clean up our mess.
clean_up () {
    greenprint "🧼 Cleaning up"
    # Remove "remote" repo.
    sudo rm -f "$IMAGE_FILENAME"
    # Remomve tmp dir.
    sudo rm -rf "$TEMPDIR"
}

##################################################
##
## RHEL8.5 custom filesystems test
##
##################################################

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "rhel85-custom-filesystem"
description = "A base system with custom mountpoints"
version = "0.0.1"

[[customizations.filesystem]]
mountpoint = "/"
size = 2147483648

[[customizations.filesystem]]
mountpoint = "/var"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/var/log"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/usr"
size = 2147483648

EOF

build_image "$BLUEPRINT_FILE" rhel85-custom-filesystem qcow2

# Download the image.
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "📥 Checking mountpoints"
INFO="$(sudo /usr/libexec/osbuild-composer-test/image-info "${IMAGE_FILENAME}")"
FAILED_MOUNTPOINTS=()

for MOUNTPOINT in '/var' '/usr' '/' '/var/log'; do
  EXISTS=$(jq -e --arg m "$MOUNTPOINT" 'any(.fstab[] | .[] == $m; .)' <<< "${INFO}")
  if ! $EXISTS; then
    FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
  fi
done

# Clean compose and blueprints.
greenprint "Clean up osbuild-composer again"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete rhel85-custom-filesystem > /dev/null

if [ ${#FAILED_MOUNTPOINTS[@]} -eq 0 ]; then
  echo "🎉 All tests passed."
  exit 0
else
  echo "🔥 One or more tests failed."
  exit 1
fi
