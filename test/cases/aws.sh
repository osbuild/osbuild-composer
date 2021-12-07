#!/bin/bash
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
AWS_INSTANCE_JSON=${TEMPDIR}/aws-instance.json
COMPOSE_START=${TEMPDIR}/compose-start-${TEST_ID}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${TEST_ID}.json
AMI_DATA=${TEMPDIR}/ami-data-${TEST_ID}.json
INSTANCE_DATA=${TEMPDIR}/instance-data-${TEST_ID}.json
INSTANCE_CONSOLE=${TEMPDIR}/instance-console-${TEST_ID}.json

SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa

# We need awscli to talk to AWS.
if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -e AWS_ACCESS_KEY_ID=${V2_AWS_ACCESS_KEY_ID} \
        -e AWS_SECRET_ACCESS_KEY=${V2_AWS_SECRET_ACCESS_KEY} \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        -v ${SSH_DATA_DIR}:${SSH_DATA_DIR}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} aws --region $AWS_REGION --output json --color on"
else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="aws --region $AWS_REGION --output json --color on"
fi
$AWS_CMD --version

# Check for the smoke test file on the AWS instance that we start.
smoke_test_check () {
    # Ensure the ssh key has restricted permissions.
    SMOKE_TEST=$(sudo ssh -i "${SSH_KEY}" redhat@"${1}" 'cat /etc/smoke-test.txt')
    if [[ $SMOKE_TEST == smoke-test ]]; then
        echo 1
    else
        echo 0
    fi
}

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

# Get the console screenshot from the AWS instance.
store_instance_screenshot () {
    INSTANCE_ID=${1}
    LOOP_COUNTER=${2}
    SCREENSHOT_FILE=${WORKSPACE}/console-screenshot-${ID}-${VERSION_ID}-${LOOP_COUNTER}.jpg

    $AWS_CMD ec2 get-console-screenshot --instance-id "${1}" > "$INSTANCE_CONSOLE"
    jq -r '.ImageData' "$INSTANCE_CONSOLE" | base64 -d - > "$SCREENSHOT_FILE"
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

# NOTE(mhayden): Getting TagSpecifications to play along with bash's
# parsing of curly braces and square brackets is nuts, so we just write some
# json and pass it to the aws command.
tee "$AWS_INSTANCE_JSON" > /dev/null << EOF
{
    "TagSpecifications": [
        {
            "ResourceType": "instance",
            "Tags": [
                {
                    "Key": "Name",
                    "Value": "${TEST_ID}"
                },
                {
                    "Key": "gitlab-ci-test",
                    "Value": "true"
                }
            ]
        }
    ]
}
EOF

# Build instance in AWS with our image.
greenprint "👷🏻 Building instance in AWS"
$AWS_CMD ec2 run-instances \
    --associate-public-ip-address \
    --image-id "${AMI_IMAGE_ID}" \
    --instance-type t3a.micro \
    --user-data file://"${SSH_DATA_DIR}"/user-data \
    --cli-input-json file://"${AWS_INSTANCE_JSON}" > /dev/null

# Wait for the instance to finish building.
greenprint "⏱ Waiting for AWS instance to be marked as running"
while true; do
    $AWS_CMD ec2 describe-instances \
        --filters Name=image-id,Values="${AMI_IMAGE_ID}" \
        | tee "$INSTANCE_DATA" > /dev/null

    INSTANCE_STATUS=$(jq -r '.Reservations[].Instances[].State.Name' "$INSTANCE_DATA")

    # Break the loop if our instance is running.
    if [[ $INSTANCE_STATUS == running ]]; then
        break
    fi

    # Sleep for 10 seconds and try again.
    sleep 10

done

# Get data about the instance we built.
INSTANCE_ID=$(jq -r '.Reservations[].Instances[].InstanceId' "$INSTANCE_DATA")
PUBLIC_IP=$(jq -r '.Reservations[].Instances[].PublicIpAddress' "$INSTANCE_DATA")

# Wait for the node to come online.
greenprint "⏱ Waiting for AWS instance to respond to ssh"
for LOOP_COUNTER in {0..30}; do
    if ssh-keyscan "$PUBLIC_IP" > /dev/null 2>&1; then
        echo "SSH is up!"
        ssh-keyscan "$PUBLIC_IP" | sudo tee -a /root/.ssh/known_hosts
        break
    fi

    # Get a screenshot of the instance console.
    echo "Getting instance screenshot..."
    store_instance_screenshot "$INSTANCE_ID" "$LOOP_COUNTER" || true

    # ssh-keyscan has a 5 second timeout by default, so the pause per loop
    # is 10 seconds when you include the following `sleep`.
    echo "Retrying in 5 seconds..."
    sleep 5
done

# Check for our smoke test file.
greenprint "🛃 Checking for smoke test file"
for LOOP_COUNTER in {0..10}; do
    RESULTS="$(smoke_test_check "$PUBLIC_IP")"
    if [[ $RESULTS == 1 ]]; then
        echo "Smoke test passed! 🥳"
        break
    fi
    sleep 5
done

# Ensure the image was properly tagged.
IMAGE_TAG=$($AWS_CMD ec2 describe-images --image-ids "${AMI_IMAGE_ID}" | jq -r '.Images[0].Tags[] | select(.Key=="Name") | .Value')
if [[ ! $IMAGE_TAG == "${TEST_ID}" ]]; then
    RESULTS=0
fi

# Clean up our mess.
greenprint "🧼 Cleaning up"
$AWS_CMD ec2 terminate-instances --instance-id "${INSTANCE_ID}"
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
