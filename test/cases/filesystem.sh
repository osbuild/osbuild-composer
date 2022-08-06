#!/usr/bin/bash

#
# Test the ability to specify custom mountpoints
#
set -euo pipefail

source /etc/os-release

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

function get_build_info() {
    key="$1"
    fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
    fi
    jq -r "${key}" "${fname}"
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
        COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")
    fi

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
        COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

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
check_result () {
    if [ ${#FAILED_MOUNTPOINTS[@]} -eq 0 ]; then
        echo "🎉 $1 scenario went as expected"
    else
        echo "🔥 $1 scenario didn't go as expected. The following mountpoints were not present:"
        printf '%s\n' "${FAILED_MOUNTPOINTS[@]}"
        exit 1
    fi
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
size = "125 MiB"

[[customizations.filesystem]]
mountpoint = "/usr"
size = 2147483648

[[customizations.filesystem]]
mountpoint = "/tmp"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/var/tmp"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/home"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/opt"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/srv"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/app"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/data"
size = 131072000
EOF

build_image "$BLUEPRINT_FILE" rhel85-custom-filesystem qcow2 false

# Download the image.
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "💬 Checking mountpoints"
INFO="$(sudo /usr/libexec/osbuild-composer-test/image-info "${IMAGE_FILENAME}")"
FAILED_MOUNTPOINTS=()

for MOUNTPOINT in '/' '/var' '/var/log' '/var/log/audit' '/var/tmp' '/usr' '/tmp' '/home' '/opt' '/srv' '/app' '/data'; do
  EXISTS=$(jq --arg m "$MOUNTPOINT" 'any(.fstab[] | .[] == $m; .)' <<< "${INFO}")
  if $EXISTS; then
    greenprint "INFO: mountpoint $MOUNTPOINT exists"
  else
    FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
  fi
done

# Check the result and pass scenario type
check_result "Passing"

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
mountpoint = "/lost+found"
size = 131072000

EOF

# build_image "$BLUEPRINT_FILE" rhel85-custom-filesystem-fail qcow2 true
build_image "$BLUEPRINT_FILE" rhel85-custom-filesystem-fail qcow2 true

# Clear the test variable
FAILED_MOUNTPOINTS=()

greenprint "💬 Checking expected failures"
for MOUNTPOINT in '/etc' '/lost+found' ; do
  if ! [[ $ERROR_MSG == *"$MOUNTPOINT"* ]]; then
    FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
  fi
done

# Check the result and pass scenario type
check_result "Failing"

# Clean compose and blueprints.
greenprint "🧼 Clean up osbuild-composer again"
sudo composer-cli blueprints delete rhel85-custom-filesystem-fail > /dev/null

clean_up

echo "🎉 All tests passed."
exit 0
