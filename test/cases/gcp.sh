#!/bin/bash

#
# Test osbuild-composer 'upload to gcp' functionality. To do so, create and
# push a blueprint with composer cli. Then, create an instance in gcp
# from the uploaded image. Finally, verify that the instance is running and
# that the package from blueprint was installed.
#

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

set -euo pipefail

if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} == 10 ]]; then
    echo "Temporary disabled b/c GCP isn't suported on el10"
    exit 1
fi

# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Check available container runtime
if which podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME=docker
else
    echo No container runtime found, install podman or docker.
    exit 2
fi

function cleanupGCP() {
    # since this function can be called at any time, ensure that we don't expand unbound variables
    GCP_CMD="${GCP_CMD:-}"
    GCP_IMAGE_NAME="${GCP_IMAGE_NAME:-}"
    GCP_INSTANCE_NAME="${GCP_INSTANCE_NAME:-}"
    GCP_ZONE="${GCP_ZONE:-}"

    if [ -n "$GCP_CMD" ]; then
        $GCP_CMD compute instances delete --zone="$GCP_ZONE" "$GCP_INSTANCE_NAME"
        $GCP_CMD compute images delete "$GCP_IMAGE_NAME"
    fi
}

TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"
    cleanupGCP
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

# Generate a string, which can be used as a predictable resource name,
# especially when running the test in CI where we may need to clean up
# resources in case the test unexpectedly fails or is canceled
CI="${CI:-false}"
if [[ "$CI" == true ]]; then
    # in CI, imitate GenerateCIArtifactName() from internal/test/helpers.go
    TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_JOB_ID"
else
    # if not running in Jenkins, generate ID not relying on specific env variables
    TEST_ID=$(uuidgen);
fi

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Set up temporary files.
GCP_CONFIG=${TEMPDIR}/gcp.toml
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
BLUEPRINT_NAME="test"
COMPOSE_START=${TEMPDIR}/compose-start-${TEST_ID}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${TEST_ID}.json
GCP_TEST_ID_HASH="$(echo -n "$TEST_ID" | sha224sum - | sed -E 's/([a-z0-9])\s+-/\1/')"
GCP_IMAGE_NAME="image-$GCP_TEST_ID_HASH"
SSH_USER="cloud-user"

# Need gcloud to talk to GCP
if ! hash gcloud; then
    echo "Using 'gcloud' from a container"
    sudo "${CONTAINER_RUNTIME}" pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # directory mounted to the container, in which gcloud stores the credentials after logging in
    GCP_CMD_CREDS_DIR="${TEMPDIR}/gcloud_credentials"
    mkdir "${GCP_CMD_CREDS_DIR}"

    GCP_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        --net=host \
        -v ${GCP_CMD_CREDS_DIR}:/root/.config/gcloud:Z \
        -v ${GOOGLE_APPLICATION_CREDENTIALS}:${GOOGLE_APPLICATION_CREDENTIALS}:Z \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} gcloud --format=json"
else
    echo "Using pre-installed 'gcloud' from the system"
    GCP_CMD="gcloud --format=json --quiet"
fi
$GCP_CMD --version

# Verify image in Compute Engine on GCP
function verifyInGCP() {
    # Authenticate
    $GCP_CMD auth activate-service-account --key-file "$GOOGLE_APPLICATION_CREDENTIALS"
    # Extract and set the default project to be used for commands
    GCP_PROJECT=$(jq -r '.project_id' "$GOOGLE_APPLICATION_CREDENTIALS")
    $GCP_CMD config set project "$GCP_PROJECT"

    # Add "gitlab-ci-test" label to the image
    $GCP_CMD compute images add-labels "$GCP_IMAGE_NAME" --labels=gitlab-ci-test=true

    # Verify that the image boots and have customizations applied
    # Create SSH keys to use
    GCP_SSH_KEY="$TEMPDIR/id_google_compute_engine"
    ssh-keygen -t rsa-sha2-512 -f "$GCP_SSH_KEY" -C "$SSH_USER" -N ""

    # create the instance
    # resource ID can have max 62 characters, the $GCP_TEST_ID_HASH contains 56 characters
    GCP_INSTANCE_NAME="vm-$GCP_TEST_ID_HASH"

    # Ensure that we use random GCP region with available 'IN_USE_ADDRESSES' quota
    # We use the CI variable "GCP_REGION" as the base for expression to filter regions.
    # It works best if the "GCP_REGION" is set to a storage multi-region, such as "us"
    local GCP_COMPUTE_REGION
    GCP_COMPUTE_REGION=$($GCP_CMD compute regions list --filter="name:$GCP_REGION* AND status=UP" | jq -r '.[] | select(.quotas[] as $quota | $quota.metric == "IN_USE_ADDRESSES" and $quota.limit > $quota.usage) | .name' | shuf -n1)

    # Randomize the used GCP zone to prevent hitting "exhausted resources" error on each test re-run
    GCP_ZONE=$($GCP_CMD compute zones list --filter="region=$GCP_COMPUTE_REGION AND status=UP" | jq -r '.[].name' | shuf -n1)

    # Pick the smallest '^n\d-standard-\d$' machine type from those available in the zone
    local GCP_MACHINE_TYPE
    GCP_MACHINE_TYPE=$($GCP_CMD compute machine-types list --filter="zone=$GCP_ZONE AND name~^n\d-standard-\d$" | jq -r '.[].name' | sort | head -1)

    $GCP_CMD compute instances create "$GCP_INSTANCE_NAME" \
        --zone="$GCP_ZONE" \
        --image-project="$GCP_PROJECT" \
        --image="$GCP_IMAGE_NAME" \
        --machine-type="$GCP_MACHINE_TYPE" \
        --labels=gitlab-ci-test=true

    HOST=$($GCP_CMD compute instances describe "$GCP_INSTANCE_NAME" --zone="$GCP_ZONE" --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

    echo "⏱ Waiting for GCP instance to respond to ssh"
    _instanceWaitSSH "$HOST"

    # Verify image
    _ssh="$GCP_CMD compute ssh --strict-host-key-checking=no --ssh-key-file=$GCP_SSH_KEY --zone=$GCP_ZONE --quiet $SSH_USER@$GCP_INSTANCE_NAME --"
    _instanceCheck "$_ssh"
}

# Wait for the instance to be available over SSH
function _instanceWaitSSH() {
    local HOST="$1"

    for LOOP_COUNTER in {0..30}; do
        if ssh-keyscan "$HOST" > /dev/null 2>&1; then
            echo "SSH is up!"
            ssh-keyscan "$HOST" | sudo tee -a /root/.ssh/known_hosts
            break
        fi
        echo "Retrying in 5 seconds... $LOOP_COUNTER"
        sleep 5
    done
}

# Check the instance
function _instanceCheck() {
    echo "✔️ Instance checking"
    local _ssh="$1"

    # Check if unbound is installed
    $_ssh rpm -q unbound
}

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-gcp.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-gcp.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Write an GCP TOML file
tee "$GCP_CONFIG" > /dev/null << EOF
provider = "gcp"

[settings]
bucket = "${GCP_BUCKET}"
region = "${GCP_REGION}"
credentials = "$(base64 -w 0 "${GOOGLE_APPLICATION_CREDENTIALS}")"
EOF

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "${BLUEPRINT_NAME}"
description = "Testing blueprint"
version = "0.0.1"

[[packages]]
name = "unbound"

[customizations.services]
enabled = ["unbound"]
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve "$BLUEPRINT_NAME"

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!

# Start the compose and upload to GCP.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start "$BLUEPRINT_NAME" gce "$GCP_IMAGE_NAME" "$GCP_CONFIG" | tee "$COMPOSE_START"
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
    echo "❌ Something went wrong with the compose. 😢"
    exit 1
fi

verifyInGCP

# Also delete the compose so we don't run out of disk space
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null

greenprint "💚 Success"
exit 0
