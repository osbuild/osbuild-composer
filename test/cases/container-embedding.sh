#!/usr/bin/bash
set -euxo pipefail


source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

#
# Provision the software under test.
#

/usr/libexec/osbuild-composer-test/provision.sh none

TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${TEMPDIR}/osbuild-${ID}-${VERSION_ID}-azure.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${TEMPDIR}/osbuild-${ID}-${VERSION_ID}-azure.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

BRANCH_NAME="${CI_COMMIT_BRANCH:-local}"
BUILD_ID="${CI_BUILD_ID:-$(uuidgen)}"
TEST_ID="$DISTRO_CODE-$ARCH-$BRANCH_NAME-$BUILD_ID"
IMAGE_KEY=container-${TEST_ID}

# Set up temporary files.
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

IMAGE_DIGEST="sha256:4d76a7480ce1861c95975945633dc9d03807ffb45c64b664ef22e673798d414b"

# Write a basic blueprint for our container.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "image"
description = "A qcwo2 with an container"
version = "0.0.1"

[[containers]]
source = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal@${IMAGE_DIGEST}"
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve image

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!
# Stop watching the worker journal when exiting.
trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

# Start the compose and upload to CI registry.
greenprint "🚀 Starting compose"

sudo composer-cli --json compose start image qcow2 | tee "$COMPOSE_START"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

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
    sleep 30
done

# Capture the compose logs from osbuild.
greenprint "💬 Getting compose log and metadata"
get_compose_log "$COMPOSE_ID"
get_compose_metadata "$COMPOSE_ID"

# Kill the journal monitor immediately and remove the trap
sudo pkill -P ${WORKER_JOURNAL_PID}
trap - EXIT

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
else
    greenprint "💚 Success!"
fi

# Download the image.
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "💬 Checking that image exists"
INFO="$(sudo /usr/libexec/osbuild-composer-test/image-info "${IMAGE_FILENAME}")"

IMAGE_ID="d4ee87dab8193afad523b1042b9d3f5ec887555a704e5aaec2876798ebb585a6"
EXISTS=$(jq -e --arg id "${IMAGE_ID}" 'any(."container-images" | select(. != null and .[].Id == $id); .)' <<< "${INFO}")

if $EXISTS; then
  greenprint "💚 container image '${IMAGE_ID}' was found!"
else
  echo "😢 container image '${IMAGE_ID}' not in image."
  exit 1
fi
