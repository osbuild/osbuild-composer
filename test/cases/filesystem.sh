#!/usr/bin/bash

#
# Test the ability to specify custom mountpoints
#
set -euo pipefail

source /etc/os-release

case "${ID}-${VERSION_ID}" in
    "rhel-8.6" | "rhel-9.0" | "centos-9")
        ;;
    *)
        echo "$0 is not enabled for ${ID}-${VERSION_ID} skipping..."
        exit 0
        ;;
esac

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
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
    want_fail=$4

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
    # this needs "|| true" at the end for the fail case scenario
    sudo composer-cli --json compose start "$blueprint_name" "$image_type" | tee "$COMPOSE_START" || true
    if rpm -q --quiet weldr-client; then
        STATUS=$(jq -r '.body.status' "$COMPOSE_START")
    else
        STATUS=$(jq -r '.status' "$COMPOSE_START")
    fi
    
    if [[ $want_fail == "$STATUS" ]]; then
        echo "Something went wrong with the compose. 😢"
        sudo pkill -P ${WORKER_JOURNAL_PID}
        trap - EXIT
        exit 1
    elif [[ $want_fail == true && $STATUS == false ]]; then
        sudo pkill -P ${WORKER_JOURNAL_PID}
        trap - EXIT
        if rpm -q --quiet weldr-client; then
            ERROR_MSG=$(jq 'first(.body.errors[] | select(.id == "ManifestCreationFailed")) | .msg' "$COMPOSE_START")
        else
            ERROR_MSG=$(jq 'first(.errors[] | select(.id == "ManifestCreationFailed")) | .msg' "$COMPOSE_START")
        fi
        return
    else
        if rpm -q --quiet weldr-client; then
            COMPOSE_ID=$(jq -r '.body.build_id' "$COMPOSE_START")
        else
            COMPOSE_ID=$(jq -r '.build_id' "$COMPOSE_START")
        fi
    fi

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
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
## Custom filesystems test - success case
##
##################################################

greenprint "🚀 Checking custom filesystems (success case)"

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
mountpoint = "/var/log/audit"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/usr"
size = 2147483648

EOF

build_image "$BLUEPRINT_FILE" rhel85-custom-filesystem qcow2 false

# Download the image.
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "💬 Checking mountpoints"
INFO="$(sudo /usr/libexec/osbuild-composer-test/image-info "${IMAGE_FILENAME}")"
FAILED_MOUNTPOINTS=()

for MOUNTPOINT in '/var' '/usr' '/' '/var/log'; do
  EXISTS=$(jq -e --arg m "$MOUNTPOINT" 'any(.fstab[] | .[] == $m; .)' <<< "${INFO}")
  if ! $EXISTS; then
    FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
  fi
done

# Clean compose and blueprints.
greenprint "🧼 Clean up osbuild-composer again"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete rhel85-custom-filesystem > /dev/null

##################################################
##
## Custom filesystems test - fail case
##
##################################################

greenprint "🚀 Checking custom filesystems (fail case)"

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "rhel85-custom-filesystem-fail"
description = "A base system with custom mountpoints"
version = "0.0.1"

[[customizations.filesystem]]
mountpoint = "/"
size = 2147483648

[[customizations.filesystem]]
mountpoint = "/etc"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/boot"
size = 131072000

EOF

# build_image "$BLUEPRINT_FILE" rhel85-custom-filesystem-fail qcow2 true
build_image "$BLUEPRINT_FILE" rhel85-custom-filesystem-fail qcow2 true

# Check error message.
FAILED_MOUNTPOINTS=()

greenprint "💬 Checking expected failures"
for MOUNTPOINT in '/etc' '/boot' ; do
  if ! [[ $ERROR_MSG == *"$MOUNTPOINT"* ]]; then
    FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
  fi
done

# Clean compose and blueprints.
greenprint "🧼 Clean up osbuild-composer again"
sudo composer-cli blueprints delete rhel85-custom-filesystem-fail > /dev/null

clean_up

if [ ${#FAILED_MOUNTPOINTS[@]} -eq 0 ]; then
  echo "🎉 All tests passed."
  exit 0
else
  echo "🔥 One or more tests failed."
  exit 1
fi
