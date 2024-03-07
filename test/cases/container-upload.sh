#!/usr/bin/bash
set -euxo pipefail


source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

#
# Provision the software under test.
#

/usr/libexec/osbuild-composer-test/provision.sh none

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-azure.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-azure.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

if [ "$ID" != "fedora" ]; then
    echo "Test is supposed to run only on Fedora"
    exit 1
fi

BRANCH_NAME="${CI_COMMIT_BRANCH:-local}"
BUILD_ID="${CI_JOB_ID:-$(uuidgen)}"
TEST_ID="$DISTRO_CODE-$ARCH-$BRANCH_NAME-$BUILD_ID"
IMAGE_KEY=container-${TEST_ID}

# Set up temporary files.
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
UPLOAD_CONFIG=${TEMPDIR}/upload.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# Write a basic blueprint for our container.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "container"
description = "A base container"
version = "0.0.1"
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve container

# Write the upload configuration.
tee "$UPLOAD_CONFIG" > /dev/null << EOF
provider = "container"

[settings]
username = "$CI_REGISTRY_USER"
password = "$CI_JOB_TOKEN"
EOF

UPLOAD_TARGET="$CI_REGISTRY_IMAGE/fedora-container:$CI_COMMIT_REF_SLUG"

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!

# Start the compose and upload to CI registry.
greenprint "🚀 Starting compose with upload to $UPLOAD_TARGET"

sudo composer-cli --json compose start container container "$UPLOAD_TARGET" "$UPLOAD_CONFIG" | tee "$COMPOSE_START"
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

# Kill the journal monitor
sudo pkill -P ${WORKER_JOURNAL_PID}

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
else
    greenprint "💚 Success!"
fi
