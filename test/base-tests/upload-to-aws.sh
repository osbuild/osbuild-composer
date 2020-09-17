#!/bin/bash
set -xeuo pipefail

source /etc/os-release

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

# Apply lorax patch to work around pytoml issues in RHEL 8.x.
# See BZ 1843704 or https://github.com/weldr/lorax/pull/1030 for more details.
if [[ $ID == rhel ]]; then
    sudo sed -r -i 's#toml.load\(args\[3\]\)#toml.load(open(args[3]))#' \
        /usr/lib/python3.6/site-packages/composer/cli/compose.py
    sudo rm -f /usr/lib/python3.6/site-packages/composer/cli/compose.pyc
fi

# We need jq for parsing composer-cli output.
if ! hash jq; then
    greenprint "Installing jq"
    sudo dnf -qy install jq
fi

TEST_ID="${CHANGE_ID}-${BUILD_ID}"
IMAGE_KEY=osbuild-composer-base-test-${TEST_ID}

# Jenkins sets WORKSPACE to the job workspace, but if this script runs
# outside of Jenkins, we can set up a temporary directory instead.
if [[ ${WORKSPACE:-empty} == empty ]]; then
    WORKSPACE=$(mktemp -d)
fi

# Set up temporary files.
TEMPDIR="$(mktemp -d)"
AWS_CONFIG="${TEMPDIR}/aws.toml"
BLUEPRINT_FILE="${TEMPDIR}/blueprint.toml"
BLUEPRINT_NAME="targetimage"
COMPOSE_START="${TEMPDIR}/compose-start-${IMAGE_KEY}.json"
COMPOSE_INFO="${TEMPDIR}/compose-info-${IMAGE_KEY}.json"

# Get the compose log.
get_compose_log () {
    COMPOSE_ID="$1"
    LOG_FILE="${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-aws.log"

    # Download the logs.
    sudo composer-cli compose log "${COMPOSE_ID}" >> "${LOG_FILE}"
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID="$1"
    METADATA_FILE="${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-aws.json"

    # Download the metadata.
    sudo composer-cli compose metadata "${COMPOSE_ID}" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    tar -xf "${TARBALL}"
    rm -f "${TARBALL}"

    # Move the JSON file into place.
    cat "${COMPOSE_ID}.json" | jq -M '.' >> "${METADATA_FILE}"
}

# Write an AWS TOML file
tee "${AWS_CONFIG}" > /dev/null << EOF
provider = "aws"

[settings]
accessKeyID = "${AWS_ACCESS_KEY_ID}"
secretAccessKey = "${AWS_SECRET_ACCESS_KEY}"
bucket = "${AWS_BUCKET}"
region = "${AWS_REGION}"
key = "${IMAGE_KEY}"
EOF

# Write a basic blueprint for our image.
tee "${BLUEPRINT_FILE}" << EOF
name = "${BLUEPRINT_NAME}"
description = "A base system with osbuild-composer"
version = "0.0.1"

[[packages]]
name = "osbuild-composer"

[customizations.services]
enabled = ["sshd", "cloud-init", "cloud-init-local", "cloud-config", "cloud-final", "osbuild-composer.socket"]
EOF

# Add osbuild-mock repo as a source to osbuild-composer
greenprint "📋 Adding sources"
sudo composer-cli sources add osbuild-mock.toml

sudo composer-cli sources list
sudo composer-cli sources info osbuild-mock

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
cat "${BLUEPRINT_FILE}"
sudo composer-cli -j blueprints push "${BLUEPRINT_FILE}"
greenprint "📋 Depsolving blueprint"
sudo composer-cli -j blueprints depsolve "${BLUEPRINT_NAME}"

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | egrep -o "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &

# Start the compose and upload to AWS.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start "${BLUEPRINT_NAME}" ami "${IMAGE_KEY}" "${AWS_CONFIG}" | tee "${COMPOSE_START}"
COMPOSE_ID=$(jq -r '.build_id' "${COMPOSE_START}")

# Wait for the compose to finish.
greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "${COMPOSE_INFO}" > /dev/null
    COMPOSE_STATUS=$(jq -r '.queue_status' "${COMPOSE_INFO}")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

# Capture the compose logs from osbuild.
greenprint "💬 Getting compose log and metadata"
get_compose_log "${COMPOSE_ID}"
get_compose_metadata "${COMPOSE_ID}"

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi

exit 0