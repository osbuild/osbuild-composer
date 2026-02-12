#!/bin/bash

#
# Test osbuild-composer 'upload to azure' functionality. To do so, create and
# push a blueprint with composer cli. Then, use terraform to create
# an instance in azure from the uploaded image. Finally, verify the instance
# is running with cloud-init ran.
#

shopt -s expand_aliases
set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

BRANCH_NAME="${CI_COMMIT_BRANCH:-local}"
BUILD_ID="${CI_JOB_ID:-$(uuidgen)}"
HYPER_V_GEN="${HYPER_V_GEN:-V1}"

# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Check available container runtime
if type -p podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME=docker
else
    echo No container runtime found, install podman or docker.
    exit 2
fi

TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

# Terraform needs azure-cli to talk to Azure.
if ! hash az; then
    echo "Using 'azure-cli' from a container"
    sudo "${CONTAINER_RUNTIME}" pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # directory mounted to the container, in which azure-cli stores the credentials after logging in
    AZURE_CMD_CREDS_DIR="${TEMPDIR}/azure-cli_credentials"
    mkdir "${AZURE_CMD_CREDS_DIR}"

    AZURE_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        --net=host \
        -v ${AZURE_CMD_CREDS_DIR}:/root/.azure:Z \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} az"
    alias az='${AZURE_CMD}'
else
    echo "Using pre-installed 'azure-cli' from the system"
fi
az version

ARCH=$(uname -m)
# Remove the potential dot from the DISTRO_CODE to workaround cloud-image-val limitation
# TODO: remove once https://github.com/osbuild/cloud-image-val/pull/290 is merged
TEST_ID="${DISTRO_CODE//./}-$ARCH-$BRANCH_NAME-$BUILD_ID"
IMAGE_KEY=image-${TEST_ID}

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Set up temporary files.
AZURE_CONFIG=${TEMPDIR}/azure.toml
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

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

# Export Azure credentials if running on Jenkins
set +u
if [ -n "$AZURE_CREDS" ]
then
    exec 4<"$AZURE_CREDS"
    readarray -t -u 4 vars
    for line in "${vars[@]}"; do export "${line?}"; done
    exec 4<&-
fi
set -u

# Write an Azure TOML file
tee "$AZURE_CONFIG" > /dev/null << EOF
provider = "azure"

[settings]
storageAccount = "${AZURE_STORAGE_ACCOUNT}"
storageAccessKey = "${AZURE_STORAGE_ACCESS_KEY}"
container = "${AZURE_CONTAINER_NAME}"
EOF

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "bash"
description = "A base system with bash"
version = "0.0.1"

[[packages]]
name = "bash"

[[packages]]
name = "cloud-init"

[customizations.services]
enabled = ["sshd", "cloud-init", "cloud-init-local", "cloud-config", "cloud-final"]
EOF

# Make sure the specified storage account exists
if [ "$(az resource list --name "$AZURE_STORAGE_ACCOUNT")" == "[]" ]; then
	echo "The storage account ${AZURE_STORAGE_ACCOUNT} was removed!"
	az storage account create \
		--name "${AZURE_STORAGE_ACCOUNT}" \
		--resource-group "${AZURE_RESOURCE_GROUP}" \
		--location  "${AZURE_LOCATION}" \
		--sku Standard_RAGRS \
		--kind StorageV2
fi

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve bash

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!

# Start the compose and upload to Azure.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start bash vhd "$IMAGE_KEY" "$AZURE_CONFIG" | tee "$COMPOSE_START"
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
    redprint "Something went wrong with the compose. 😢"
    exit 1
fi

export BLOB_URL="https://$AZURE_STORAGE_ACCOUNT.blob.core.windows.net/$AZURE_CONTAINER_NAME/$IMAGE_KEY.vhd"

greenprint "Pulling cloud-image-val container"

if [[ "$CI_PROJECT_NAME" =~ "cloud-image-val" ]]; then
  # If running on CIV, get dev container
  TAG=${CI_COMMIT_REF_SLUG}
elif ! nvrGreaterOrEqual "osbuild-composer" "151"; then
  # osbuild-composer v151 made changes in the Azure image definitions and CIV
  # was updated to check for those changes:
  # https://github.com/osbuild/cloud-image-val/pull/457
  # This tag does not include those changes, so use it when running against
  # older versions of composer.
  TAG="pr-456"
else
  # If not, get prod container
  TAG="prod"
fi

CONTAINER_CLOUD_IMAGE_VAL="quay.io/cloudexperience/cloud-image-val:$TAG"

sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_CLOUD_IMAGE_VAL}"

greenprint "Running cloud-image-val on generated image"

tee "${TEMPDIR}/resource-file.json" <<EOF
{
  "subscription_id": "${AZURE_SUBSCRIPTION_ID}",
  "resource_group": "${AZURE_RESOURCE_GROUP}",
  "provider": "azure",
  "instances": [
    {
      "vhd_uri": "${BLOB_URL}",
      "arch": "${ARCH}",
      "location": "${AZURE_LOCATION}",
      "name": "${IMAGE_KEY}",
      "hyper_v_generation": "${HYPER_V_GEN}",
      "storage_account": "${AZURE_STORAGE_ACCOUNT}"
    }
  ]
}
EOF

if [ -z "$CIV_CONFIG_FILE" ]; then
    redprint "ERROR: please provide the variable CIV_CONFIG_FILE"
    exit 1
fi

cp "${CIV_CONFIG_FILE}" "${TEMPDIR}/civ_config.yml"

# temporary workaround for
# https://issues.redhat.com/browse/CLOUDX-488
if nvrGreaterOrEqual "osbuild-composer" "83"; then
    sudo "${CONTAINER_RUNTIME}" run \
        --net=host \
        -a stdout -a stderr \
        -e ARM_CLIENT_ID="${V2_AZURE_CLIENT_ID}" \
        -e ARM_CLIENT_SECRET="${V2_AZURE_CLIENT_SECRET}" \
        -e ARM_SUBSCRIPTION_ID="${AZURE_SUBSCRIPTION_ID}" \
        -e ARM_TENANT_ID="${AZURE_TENANT_ID}" \
        -e JIRA_PAT="${JIRA_PAT}" \
        -v "${TEMPDIR}":/tmp:Z \
        "${CONTAINER_CLOUD_IMAGE_VAL}" \
        python cloud-image-val.py \
        -c /tmp/civ_config.yml \
        && RESULTS=1 || RESULTS=0

    mv "${TEMPDIR}"/report.html "${ARTIFACTS}"
else
    RESULTS=1
fi

# Also delete the compose so we don't run out of disk space
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null

# Use the return code of the smoke test to determine if we passed or failed.
if [[ $RESULTS == 1 ]]; then
    greenprint "💚 Success with HyperV ${HYPER_V_GEN}"
    exit 0
elif [[ $RESULTS != 1 ]]; then
    redprint "❌ Failed ${HYPER_V_GEN}"
    exit 1
fi

exit 0
