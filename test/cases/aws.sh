#!/bin/bash

#
# Test osbuild-composer 'upload to aws' functionality. To do so, create and
# push a blueprint with composer cli. Then, create an instance in aws 
# from the uploaded image. Finally, verify that the instance is running and 
# cloud init ran.
#

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

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

# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Check available container runtime
if which podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME=docker
else
    echo No container runtime found, install podman or docker.
    exit 2
fi

TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

# Generate a string, which can be used as a predictable resource name,
# especially when running the test in CI where we may need to clean up
# resources in case the test unexpectedly fails or is canceled
CI="${CI:-false}"
if [[ "$CI" == true ]]; then
  # in CI, imitate GenerateCIArtifactName() from internal/test/helpers.go
  TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_BUILD_ID"
else
  # if not running in Jenkins, generate ID not relying on specific env variables
  TEST_ID=$(uuidgen);
fi


# Jenkins sets WORKSPACE to the job workspace, but if this script runs
# outside of Jenkins, we can set up a temporary directory instead.
if [[ ${WORKSPACE:-empty} == empty ]]; then
    WORKSPACE=$(mktemp -d)
fi

# Set up temporary files.
AWS_CONFIG=${TEMPDIR}/aws.toml
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${TEST_ID}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${TEST_ID}.json
AMI_DATA=${TEMPDIR}/ami-data-${TEST_ID}.json

# We need awscli to talk to AWS.
if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -e AWS_ACCESS_KEY_ID=${V2_AWS_ACCESS_KEY_ID} \
        -e AWS_SECRET_ACCESS_KEY=${V2_AWS_SECRET_ACCESS_KEY} \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} aws --region $AWS_REGION --output json --color on"
else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="aws --region $AWS_REGION --output json --color on"
fi
$AWS_CMD --version

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-aws.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-aws.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

is_weldr_client_installed () {
    if rpm --quiet -q weldr-client; then
        echo true
    else
        echo false
    fi
}

# Write an AWS TOML file
tee "$AWS_CONFIG" > /dev/null << EOF
provider = "aws"

[settings]
accessKeyID = "${V2_AWS_ACCESS_KEY_ID}"
secretAccessKey = "${V2_AWS_SECRET_ACCESS_KEY}"
bucket = "${AWS_BUCKET}"
region = "${AWS_REGION}"
key = "${TEST_ID}"
EOF

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "bash"
description = "A base system with bash"
version = "0.0.1"

[[packages]]
name = "bash"

[customizations.services]
enabled = ["sshd", "cloud-init", "cloud-init-local", "cloud-config", "cloud-final"]
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve bash

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!
# Stop watching the worker journal when exiting.
trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

# Start the compose and upload to AWS.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start bash ami "$TEST_ID" "$AWS_CONFIG" | tee "$COMPOSE_START"
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

# Find the image that we made in AWS.
greenprint "🔍 Search for created AMI"
$AWS_CMD ec2 describe-images \
    --owners self \
    --filters Name=name,Values="${TEST_ID}" \
    | tee "$AMI_DATA" > /dev/null

AMI_IMAGE_ID=$(jq -r '.Images[].ImageId' "$AMI_DATA")
SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' "$AMI_DATA")

# Tag image and snapshot with "gitlab-ci-test" tag
$AWS_CMD ec2 create-tags \
    --resources "${SNAPSHOT_ID}" "${AMI_IMAGE_ID}" \
    --tags Key=gitlab-ci-test,Value=true

if [[ "$ID" == "fedora" ]]; then
  # fedora uses fedora
  SSH_USER="fedora"
  DISTRO_HASHICORP_REPO="fedora"
else
  # RHEL and centos use ec2-user
  SSH_USER="ec2-user"
  DISTRO_HASHICORP_REPO="RHEL"
fi

greenprint "Cloning cloud-image-val from upstream"

release_version="v0.1.0"
git clone https://github.com/osbuild/cloud-image-val --branch "${release_version}"

greenprint "Running cloud-image-val on generated image"

pushd cloud-image-val

sudo dnf install -y python3-pip unzip make dnf-plugins-core

sudo dnf config-manager --add-repo https://rpm.releases.hashicorp.com/"${DISTRO_HASHICORP_REPO}"/hashicorp.repo
sudo dnf -y install terraform

sudo pip3 install --upgrade pip
sudo pip3 install -r requirements.txt

tee "resource-file.json" <<EOF
{
    "provider": "aws",
    "instances": [
        {
            "ami": "$AMI_IMAGE_ID",
            "region": "us-east-1",
            "instance_type": "t3a.micro",
            "username": "$SSH_USER",
            "name": "testing-image"
        }
    ]
}
EOF

AWS_ACCESS_KEY_ID=${V2_AWS_ACCESS_KEY_ID} \
AWS_SECRET_ACCESS_KEY=${V2_AWS_SECRET_ACCESS_KEY} \
python3 cloud-image-val.py -r resource-file.json -d -o report.xml -m 'not pub' && RESULTS=1 || RESULTS=0

popd

# Clean up our mess.
greenprint "🧼 Cleaning up"
$AWS_CMD ec2 deregister-image --image-id "${AMI_IMAGE_ID}"
$AWS_CMD ec2 delete-snapshot --snapshot-id "${SNAPSHOT_ID}"

# Also delete the compose so we don't run out of disk space
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null

# Use the return code of the smoke test to determine if we passed or failed.
# On rhel continue with the cloudapi test
if [[ $RESULTS == 1 ]]; then
    greenprint "💚 Success"
    exit 0
elif [[ $RESULTS != 1 ]]; then
    greenprint "❌ Failed"
    exit 1
fi

exit 0
