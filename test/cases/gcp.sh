#!/bin/bash
set -euxo pipefail

source /etc/os-release

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

#TODO: Remove this once there is rhel9 support for GCP image type
if [[ $DISTRO_CODE == rhel_90 ]]; then
    greenprint "Skipped"
    exit 0
fi

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

# Apply lorax patch to work around pytoml issues in RHEL 8.x.
# See BZ 1843704 or https://github.com/weldr/lorax/pull/1030 for more details.
if [[ $ID == rhel ]]; then
    sudo sed -r -i 's#toml.load\(args\[3\]\)#toml.load(open(args[3]))#' \
        /usr/lib/python3.6/site-packages/composer/cli/compose.py
    sudo rm -f /usr/lib/python3.6/site-packages/composer/cli/compose.pyc
fi

WORKDIR=$(mktemp -d)
function cleanup() {
    greenprint "🧼 Cleaning up"

    COMPOSE_ID="${COMPOSE_ID:-}"
    if [ -n "$COMPOSE_ID" ]; then
        sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
    fi

    rm -rf "$WORKDIR"
}
trap cleanup EXIT

# Generate a string, which can be used as a predictable resource name,
# especially when running the test in Jenkins where we may need to clean up
# resources in case the test unexpectedly fails or is canceled
JENKINS_HOME="${JENKINS_HOME:-}"
if [[ -n "$JENKINS_HOME" ]]; then
    # in Jenkins, imitate GenerateCIArtifactName() from internal/test/helpers.go
    TEST_ID="$DISTRO_CODE-$ARCH-$BRANCH_NAME-$BUILD_ID"
else
    # if not running in Jenkins, generate ID not relying on specific env variables
    TEST_ID=$(uuidgen);
fi

# constrains for GCP resource IDs:
# - max 62 characters
# - must be a match of regex '[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?|[1-9][0-9]{0,19}'
#
# use sha224sum to get predictable 56 characters long testID without invalid characters
GCP_TEST_ID_HASH="$(echo -n "$TEST_ID" | sha224sum - | sed -E 's/([a-z0-9])\s+-/\1/')"
GCP_IMAGE_NAME="image-$GCP_TEST_ID_HASH"

# Set up temporary files.
BLUEPRINT_FILE=${WORKDIR}/blueprint.toml
COMPOSE_START=${WORKDIR}/compose-start-${GCP_IMAGE_NAME}.json
COMPOSE_INFO=${WORKDIR}/compose-info-${GCP_IMAGE_NAME}.json

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${WORKDIR}/osbuild-${ID}-${VERSION_ID}-gcp.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${WORKDIR}/osbuild-${ID}-${VERSION_ID}-gcp.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    tar -xf "$TARBALL"
    rm -f "$TARBALL"

    # Move the JSON file into place.
    cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "postgresql"
description = "A base system with postgresql"
version = "0.0.1"

[[packages]]
name = "postgresql"

[customizations.services]
enabled = ["sshd"]
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve postgresql

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!

# Start the compose.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start postgresql gce-byos | tee "$COMPOSE_START"
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
    sleep 30
done

# Capture the compose logs from osbuild.
greenprint "💬 Getting compose log and metadata"
get_compose_log "$COMPOSE_ID"
get_compose_metadata "$COMPOSE_ID"

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi

# Stop watching the worker journal.
sudo kill ${WORKER_JOURNAL_PID}

greenprint "💚 Success"
exit 0
