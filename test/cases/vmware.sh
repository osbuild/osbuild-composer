#!/bin/bash

#
# Test osbuild-composer 'upload to vmware' functionality. To do so, create and
# push a blueprint with composer cli. Then, create an instance in vSphere
# from the uploaded image. Finally, verify that the instance is running and 
# cloud init ran.
#

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh


IMAGE_TYPE="$1"

if ! nvrGreaterOrEqual "osbuild-composer" "84" && [ "$IMAGE_TYPE" == "ova" ]; then
    greenprint "Skipping ova test on older osbuild-composer"
    exit 0
fi

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

GOVC_CMD=/tmp/govc

# Note: in GitLab CI the GOVMOMI_ variables are defined one-by-one
# instead of sourcing them from a file!
VCENTER_CREDS="${VCENTER_CREDS:-}"
if [ -n "$VCENTER_CREDS" ]; then
# shellcheck source=/dev/null
    source "$VCENTER_CREDS"
fi

# We need govc to talk to vSphere
if ! hash govc; then
    greenprint "Installing govc"
    pushd /tmp
        curl -Ls --retry 5 --output govc.gz \
            https://github.com/vmware/govmomi/releases/download/v0.24.0/govc_linux_amd64.gz
        gunzip -f govc.gz
        chmod +x /tmp/govc
        $GOVC_CMD version
    popd
fi

# Generate a string, which can be used as a predictable resource name,
# which helps identify issues and link lefotever resources to PRs
CI="${CI:-false}"
if [[ "$CI" == true ]]; then
  TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_JOB_ID"
else
  TEST_ID=$(uuidgen);
fi

IMAGE_KEY=osbuild-composer-vmware-test-${TEST_ID}

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Set up temporary files
TEMPDIR=$(mktemp -d)
VMWARE_CONFIG=${TEMPDIR}/vmware.toml
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "$SSH_KEY".pub)

# destroy VMs
function cleanup() {
    set +eu
    greenprint "🧼 Cleaning up"
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"
    $GOVC_CMD vm.destroy -u "${GOVMOMI_USERNAME}":"${GOVMOMI_PASSWORD}"@"${GOVMOMI_URL}" -k=true "${IMAGE_KEY}"
    set -eu
}
trap cleanup EXIT


# Check that the system started and is running correctly
running_test_check () {
    STATUS=$(sudo ssh -i "${SSH_KEY}" redhat@"${1}" 'systemctl --wait is-system-running')
    if [[ $STATUS == running || $STATUS == degraded ]]; then
        echo 0
    else
        echo 1
    fi
}

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-vmware.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-vmware.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Write an VMWare TOML file
tee "$VMWARE_CONFIG" > /dev/null << EOF
provider = "vmware"

[settings]
host = "${GOVMOMI_URL}"
username = "${GOVMOMI_USERNAME}"
password = "${GOVMOMI_PASSWORD}"
cluster = "${GOVMOMI_CLUSTER}"
dataStore = "${GOVMOMI_DATASTORE}"
dataCenter = "${GOVMOMI_DATACENTER}"
folder = "${GOVMOMI_FOLDER}"
EOF

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "bash"
description = "A base system with bash"
version = "0.0.1"

[[packages]]
name = "bash"

# Related RHBZ#2065734
[[packages]]
name = "ipa-client"
version = "*"

[customizations.services]
enabled = ["sshd"]

[[customizations.user]]
name = "redhat"
key = "${SSH_KEY_PUB}"
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve bash

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!

# Start the compose and upload to VMWare.
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start bash "$IMAGE_TYPE" "$IMAGE_KEY" "$VMWARE_CONFIG" | tee "$COMPOSE_START"
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
fi

if [ "$IMAGE_TYPE" = "vmdk" ]; then
greenprint "👷🏻 Building VM in vSphere"
$GOVC_CMD vm.create -u "${GOVMOMI_USERNAME}":"${GOVMOMI_PASSWORD}"@"${GOVMOMI_URL}" \
    -k=true \
    -pool="${GOVMOMI_CLUSTER}"/Resources \
    -dc="${GOVMOMI_DATACENTER}" \
    -ds="${GOVMOMI_DATASTORE}" \
    -folder="${GOVMOMI_FOLDER}" \
    -net="${GOVMOMI_NETWORK}" \
    -net.adapter=vmxnet3 \
    -m=4096 -c=2 -g=rhel8_64Guest -on=true -firmware=efi \
    -disk="${IMAGE_KEY}"/"${IMAGE_KEY}".vmdk \
    --disk.controller=scsi \
    "${IMAGE_KEY}"
elif [ "$IMAGE_TYPE" = "ova" ]; then
greenprint "👷🏻 Modifying network of the VM in vSphere"
$GOVC_CMD vm.network.add -u "${GOVMOMI_USERNAME}":"${GOVMOMI_PASSWORD}"@"${GOVMOMI_URL}" \
    -k=true \
    -net="${GOVMOMI_NETWORK}" \
    -net.adapter=vmxnet3 \
    -vm="${IMAGE_KEY}" \
    -net="${GOVMOMI_NETWORK}"

# start the vm
greenprint "👷🏻 Powering on the VM"
$GOVC_CMD vm.power -u "${GOVMOMI_USERNAME}":"${GOVMOMI_PASSWORD}"@"${GOVMOMI_URL}" \
    -k=true \
    -wait=true \
    -on \
    "${IMAGE_KEY}"

fi

# tagging vm as testing object
$GOVC_CMD tags.attach -u "${GOVMOMI_USERNAME}":"${GOVMOMI_PASSWORD}"@"${GOVMOMI_URL}" \
    -k=true \
    -c "osbuild-composer testing" gitlab-ci-test \
    "/${GOVMOMI_DATACENTER}/vm/${GOVMOMI_FOLDER}/${IMAGE_KEY}"

greenprint "Getting IP of created VM"
VM_IP=$($GOVC_CMD vm.ip -u "${GOVMOMI_USERNAME}":"${GOVMOMI_PASSWORD}"@"${GOVMOMI_URL}" -k=true -v4=true "${IMAGE_KEY}")

# Wait for the node to come online.
greenprint "⏱ Waiting for VM to respond to ssh"
LOOP_COUNTER=1
while [ $LOOP_COUNTER -le 30 ]; do
    if ssh-keyscan "$VM_IP" > /dev/null 2>&1; then
        echo "SSH is up!"
        ssh-keyscan "$VM_IP" | sudo tee -a /root/.ssh/known_hosts
        break
    fi

    # ssh-keyscan has a 5 second timeout by default, so the pause per loop
    # is 10 seconds when you include the following `sleep`.
    echo "Retrying in 5 seconds..."
    sleep 5

    ((LOOP_COUNTER++))
done

greenprint "🛃 Checking that system is running"
for LOOP_COUNTER in {0..10}; do
    RESULT="$(running_test_check "$VM_IP")"
    if [[ $RESULT == 0 ]]; then
        echo "System is running! 🥳"
        greenprint "💚 Success"
        exit 0
    fi
    sleep 5
done

greenprint "❌ Failure"
exit 1
