#!/bin/bash
set -euo pipefail

source /etc/os-release

# Fedora 31's composer-cli doesn't support doing image uploads.
if [[ $ID == fedora ]] && [[ $VERSION_ID == 31 ]]; then
    echo "Fedora 31 does not support image uploads with composer-cli."
    exit 0
fi

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

# We need awscli to talk to AWS.
if ! hash aws; then
    greenprint "Installing awscli"
    sudo dnf -y install unzip
    pushd /tmp
        curl -Ls --retry 5 --output awscliv2.zip \
            https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip
        unzip awscliv2.zip > /dev/null
        sudo ./aws/install > /dev/null
        aws --version
    popd
fi

TEST_UUID=$(uuidgen)
IMAGE_KEY=osbuild-composer-aws-test-${TEST_UUID}
AWS_CMD="aws --region $AWS_REGION --output json --color on"

# Jenkins sets WORKSPACE to the job workspace, but if this script runs
# outside of Jenkins, we can set up a temporary directory instead.
if [[ ${WORKSPACE:-empty} == empty ]]; then
    WORKSPACE=$(mktemp -d)
fi

# Set up temporary files.
TEMPDIR=$(mktemp -d)
AWS_CONFIG=${TEMPDIR}/aws.toml
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
AWS_INSTANCE_JSON=${TEMPDIR}/aws-instance.json
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json
AMI_DATA=${TEMPDIR}/ami-data-${IMAGE_KEY}.json
INSTANCE_DATA=${TEMPDIR}/instance-data-${IMAGE_KEY}.json
INSTANCE_CONSOLE=${TEMPDIR}/instance-console-${IMAGE_KEY}.json

# Check for the smoke test file on the AWS instance that we start.
smoke_test_check () {
    # Ensure the ssh key has restricted permissions.
    SSH_KEY=${WORKSPACE}/test/keyring/id_rsa
    chmod 0600 $SSH_KEY

    SMOKE_TEST=$(ssh -i ${SSH_KEY} redhat@${1} 'cat /etc/smoke-test.txt')
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
    sudo composer-cli compose log $COMPOSE_ID | tee $LOG_FILE > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-aws.json

    # Download the metadata.
    sudo composer-cli compose metadata $COMPOSE_ID > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename $(find . -maxdepth 1 -type f -name "*-metadata.tar"))
    tar -xf $TARBALL
    rm -f $TARBALL

    # Move the JSON file into place.
    cat ${COMPOSE_ID}.json | jq -M '.' | tee $METADATA_FILE > /dev/null
}

# Get the console screenshot from the AWS instance.
store_instance_screenshot () {
    INSTANCE_ID=${1}
    LOOP_COUNTER=${2}
    SCREENSHOT_FILE=${WORKSPACE}/console-screenshot-${ID}-${VERSION_ID}-${LOOP_COUNTER}.jpg

    $AWS_CMD ec2 get-console-screenshot --instance-id ${1} > $INSTANCE_CONSOLE
    jq -r '.ImageData' $INSTANCE_CONSOLE | base64 -d - > $SCREENSHOT_FILE
}

# Write an AWS TOML file
tee $AWS_CONFIG > /dev/null << EOF
provider = "aws"

[settings]
accessKeyID = "${AWS_ACCESS_KEY_ID}"
secretAccessKey = "${AWS_SECRET_ACCESS_KEY}"
bucket = "${AWS_BUCKET}"
region = "${AWS_REGION}"
key = "${IMAGE_KEY}"
EOF

# Write a basic blueprint for our image.
tee $BLUEPRINT_FILE > /dev/null << EOF
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
sudo composer-cli blueprints push $BLUEPRINT_FILE
sudo composer-cli blueprints depsolve bash

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | egrep -o "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u ${WORKER_UNIT} &
WORKER_JOURNAL_PID=$!

# Start the compose and upload to AWS.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start bash ami $IMAGE_KEY $AWS_CONFIG | tee $COMPOSE_START
COMPOSE_ID=$(jq -r '.build_id' $COMPOSE_START)

# Wait for the compose to finish.
greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info ${COMPOSE_ID} | tee $COMPOSE_INFO > /dev/null
    COMPOSE_STATUS=$(jq -r '.queue_status' $COMPOSE_INFO)

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

# Capture the compose logs from osbuild.
greenprint "💬 Getting compose log and metadata"
get_compose_log $COMPOSE_ID
get_compose_metadata $COMPOSE_ID

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi

# Find the image that we made in AWS.
greenprint "🔍 Search for created AMI"
$AWS_CMD ec2 describe-images \
    --owners self \
    --filters Name=name,Values=${IMAGE_KEY} \
    | tee $AMI_DATA > /dev/null

AMI_IMAGE_ID=$(jq -r '.Images[].ImageId' $AMI_DATA)

# Stop watching the worker journal.
sudo kill ${WORKER_JOURNAL_PID}

# NOTE(mhayden): Getting TagSpecifications to play along with bash's
# parsing of curly braces and square brackets is nuts, so we just write some
# json and pass it to the aws command.
tee $AWS_INSTANCE_JSON > /dev/null << EOF
{
    "TagSpecifications": [
        {
            "ResourceType": "instance",
            "Tags": [
                {
                    "Key": "Name",
                    "Value": "${IMAGE_KEY}"
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
    --key-name personal_servers \
    --image-id ${AMI_IMAGE_ID} \
    --instance-type t3a.micro \
    --user-data file://${WORKSPACE}/test/cloud-init/user-data \
    --cli-input-json file://${AWS_INSTANCE_JSON} > /dev/null

# Wait for the instance to finish building.
greenprint "⏱ Waiting for AWS instance to be marked as running"
while true; do
    $AWS_CMD ec2 describe-instances \
        --filters Name=image-id,Values=${AMI_IMAGE_ID} \
        | tee $INSTANCE_DATA > /dev/null

    INSTANCE_STATUS=$(jq -r '.Reservations[].Instances[].State.Name' $INSTANCE_DATA)

    # Break the loop if our instance is running.
    if [[ $INSTANCE_STATUS == running ]]; then
        break
    fi

    # Sleep for 10 seconds and try again.
    sleep 10

done

# Get data about the instance we built.
INSTANCE_ID=$(jq -r '.Reservations[].Instances[].InstanceId' $INSTANCE_DATA)
PUBLIC_IP=$(jq -r '.Reservations[].Instances[].PublicIpAddress' $INSTANCE_DATA)

# Wait for the node to come online.
greenprint "⏱ Waiting for AWS instance to respond to ssh"
for LOOP_COUNTER in {0..30}; do
    if ssh-keyscan $PUBLIC_IP 2>&1 > /dev/null; then
        echo "SSH is up!"
        ssh-keyscan $PUBLIC_IP >> ~/.ssh/known_hosts
        break
    fi

    # Get a screenshot of the instance console.
    echo "Getting instance screenshot..."
    store_instance_screenshot $INSTANCE_ID $LOOP_COUNTER || true

    # ssh-keyscan has a 5 second timeout by default, so the pause per loop
    # is 10 seconds when you include the following `sleep`.
    echo "Retrying in 5 seconds..."
    sleep 5
done

# Check for our smoke test file.
greenprint "🛃 Checking for smoke test file"
for LOOP_COUNTER in {0..10}; do
    RESULTS="$(smoke_test_check $PUBLIC_IP)"
    if [[ $RESULTS == 1 ]]; then
        echo "Smoke test passed! 🥳"
        break
    fi
    sleep 5
done

# Clean up our mess.
greenprint "🧼 Cleaning up"
SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' $AMI_DATA)
$AWS_CMD ec2 terminate-instances --instance-id ${INSTANCE_ID}
$AWS_CMD ec2 deregister-image --image-id ${AMI_IMAGE_ID}
$AWS_CMD ec2 delete-snapshot --snapshot-id ${SNAPSHOT_ID}

# Use the return code of the smoke test to determine if we passed or failed.
if [[ $RESULTS == 1 ]]; then
  greenprint "💚 Success"
else
  greenprint "❌ Failed"
  exit 1
fi

exit 0