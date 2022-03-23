#!/bin/bash
set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

TEST_ID=${1}
S3_PROVIDER_CONFIG_FILE=${2}
S3_CMD=${3}
IMAGE_OBJECT_KEY=${4}
S3_CA_BUNDLE=${5:-""}

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

TEMPDIR=$(mktemp -d)
function cleanup() {
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Set up temporary files.
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
BLUEPRINT_NAME=empty
COMPOSE_START=${TEMPDIR}/compose-start-${TEST_ID}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${TEST_ID}.json

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-aws.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-aws.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "${BLUEPRINT_NAME}"
description = "A base system with bash"
version = "0.0.1"
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve ${BLUEPRINT_NAME}

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!
# Stop watching the worker journal when exiting.
trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

# Start the compose and upload to AWS.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start ${BLUEPRINT_NAME} qcow2 "$TEST_ID" "$S3_PROVIDER_CONFIG_FILE" | tee "$COMPOSE_START"
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
fi

# Delete the compose so we don't run out of disk space
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null

# Find the image that we made in the AWS Bucket
greenprint "🔍 Search for created image"
if ! bash -c "${S3_CMD} ls ${IMAGE_OBJECT_KEY}"; then
    echo "Failed to find the image in the S3 Bucket"
    exit 1
fi

function removeImageFromS3() {
    bash -c "${S3_CMD} rm s3://${IMAGE_OBJECT_KEY}"
}
trap removeImageFromS3 EXIT

# Generate a URL for the image
QCOW2_IMAGE_URL=$(bash -c "${S3_CMD} presign ${IMAGE_OBJECT_KEY}")

# Run the image on KVM
/usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2 bios "${QCOW2_IMAGE_URL}" "${S3_CA_BUNDLE}"
