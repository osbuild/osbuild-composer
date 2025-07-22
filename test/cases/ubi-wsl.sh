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

# Set up temporary files.
TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="

    $AZURE_CMD vm show \
       --resource-group "$AZURE_RESOURCE_GROUP" \
       --name "wsl-vm-$TEST_ID" \
       --show-details > "$TEMPDIR/vm_details.json"

    VM_ID=$(jq -r '.id' "$TEMPDIR"/vm_details.json)
    OSDISK_ID=$(jq -r '.storageProfile.osDisk.managedDisk.id' "$TEMPDIR"/vm_details.json)
    NIC_ID=$(jq -r '.networkProfile.networkInterfaces[0].id' "$TEMPDIR"/vm_details.json)
    $AZURE_CMD network nic show --ids "$NIC_ID" > "$TEMPDIR"/nic_details.json
    NSG_ID=$(jq -r '.networkSecurityGroup.id' "$TEMPDIR"/nic_details.json)
    PUBLICIP_ID=$(jq -r '.ipConfigurations[0].publicIPAddress.id' "$TEMPDIR"/nic_details.json)

    $AZURE_CMD resource delete --no-wait --ids "$VM_ID" "$OSDISK_ID" "$NIC_ID" "$NSG_ID" "$PUBLICIP_ID"
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

get_compose_image () {
    COMPOSE_ID=$1

    sudo composer-cli compose results "$COMPOSE_ID"

    TARBALL="$COMPOSE_ID.tar"
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"
}

BRANCH_NAME="${CI_COMMIT_BRANCH:-local}"
BUILD_ID="${CI_JOB_ID:-$(uuidgen)}"
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
ARCH=$(uname -m)
TEST_ID="$DISTRO_CODE-$ARCH-$BRANCH_NAME-$BUILD_ID"
COMPOSE_START=${TEMPDIR}/compose-start.json
COMPOSE_INFO=${TEMPDIR}/compose-info.json
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "wsl"
description = "wsl image"
version = "0.0.1"
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"


greenprint "🚀 Starting compose"
sudo composer-cli --json compose start wsl wsl | tee "$COMPOSE_START"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 5 seconds and try again.
    sleep 5
done

# Capture the compose logs from osbuild.
greenprint "💬 Getting compose log and metadata"
get_compose_log "$COMPOSE_ID"
get_compose_metadata "$COMPOSE_ID"

greenprint "📀 Getting disk image"
get_compose_image "$COMPOSE_ID"

DISK="$COMPOSE_ID-image.wsl"

# backward compatibility for RHEL nightly tests
if ! nvrGreaterOrEqual "osbuild-composer" "146"; then
    DISK="$COMPOSE_ID-disk.tar.gz"
fi

greenprint "Looking for disk image: $DISK"

if [ ! -f "$DISK" ]; then
    redprint "Disk image missing from results"
    exit 1
fi

if ! hash az; then
    echo "Using 'azure-cli' from a container"
    sudo "${CONTAINER_RUNTIME}" pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # directory mounted to the container, in which azure-cli stores the credentials after logging in
    AZURE_CMD_CREDS_DIR="${TEMPDIR}/azure-cli_credentials"
    mkdir "${AZURE_CMD_CREDS_DIR}"

    AZURE_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -v ${AZURE_CMD_CREDS_DIR}:/root/.azure:Z \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} az"
else
    echo "Using pre-installed 'azure-cli' from the system"
fi

# Log into Azure
function cloud_login() {
  set +x
  $AZURE_CMD login --service-principal --username "${V2_AZURE_CLIENT_ID}" --password "${V2_AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"
  set -x
}

cloud_login

$AZURE_CMD version

# Create a windows VM from the WSL snapshot image
if ! $AZURE_CMD snapshot show --name "$AZURE_WSL_SNAPSHOT_2" --resource-group "$AZURE_RESOURCE_GROUP"; then
    redprint "WSL snapshot missing from test resource group"
    exit 1
fi

#Create a new Managed Disks using the snapshot Id
AZ_DISK="$TEST_ID-wsl-disk"
$AZURE_CMD disk create \
   --hyper-v-generation V2 \
   --resource-group "$AZURE_RESOURCE_GROUP" \
   --name "$AZ_DISK" \
   --sku "Standard_LRS" \
   --location "$AZURE_WSL_LOCATION" \
   --size-gb 128 \
   --source "$AZURE_WSL_SNAPSHOT_2"

# Create VM by attaching created managed disks as OS
# The VM needs to support virtualization, supposedly all v4 and v5's support this but this wasn't
# found to be entirely reliable. The v5 AMD machines seem to support it.
$AZURE_CMD vm create \
   --resource-group "$AZURE_RESOURCE_GROUP" \
   --name "wsl-vm-$TEST_ID" \
   --attach-os-disk "$AZ_DISK" \
   --security-type "TrustedLaunch" \
   --public-ip-sku Standard \
   --location "$AZURE_WSL_LOCATION" \
    --nic-delete-option delete \
    --os-disk-delete-option delete \
    --os-type windows \
    --size "Standard_D2as_v5"

$AZURE_CMD vm open-port --resource-group "$AZURE_RESOURCE_GROUP" --name "wsl-vm-$TEST_ID" --port 22

greenprint "🛃 Wait until the VM has a public IP"
for LOOP_COUNTER in {0..30}; do
    HOST=$($AZURE_CMD vm show \
       --show-details \
       --resource-group "$AZURE_RESOURCE_GROUP" \
       --name "wsl-vm-$TEST_ID" \
       --query "publicIps" \
       --output tsv)

    if echo "$HOST" | grep -Eq "^([0-9]{1,3}[\.]){3}[0-9]{1,3}$"; then
        break
    fi
    if [ "$LOOP_COUNTER" = "30" ]; then
        redprint "👻 the VM wasn't assigned a valid ipv4 address"
        exit 1
    fi
    sleep 10
done

greenprint "🛃 Wait until sshd is up"
for LOOP_COUNTER in {0..60}; do
    if ssh-keyscan "$HOST" > /dev/null 2>&1; then
        greenprint "up!"
        break
    fi
    echo "Retrying in 10 seconds... $LOOP_COUNTER"
    sleep 10
done

sudo chmod 600 "$AZ_WSL_HOST_PRIVATE_KEY"
sudo scp -i "$AZ_WSL_HOST_PRIVATE_KEY" -o StrictHostKeyChecking=no "$DISK" "$AZURE_WSL_USER@$HOST:"

# Use absolute path to wsl.exe to avoid "The file cannot be accessed by the system."
ssh -i "$AZ_WSL_HOST_PRIVATE_KEY" -o StrictHostKeyChecking=no "$AZURE_WSL_USER@$HOST" \
    '"C:\Program Files\WSL\wsl.exe"' --import ibwsl ibwsl "$DISK"

UNAME=$(ssh -i "$AZ_WSL_HOST_PRIVATE_KEY" -o StrictHostKeyChecking=no "$AZURE_WSL_USER@$HOST" '"C:\Program Files\WSL\wsl.exe"' -d ibwsl uname)
if [ ! "$UNAME" = "Linux" ]; then
    redprint "Not running linux on the windows host :("
    exit 1
fi

OS_RELEASE=$(ssh -i "$AZ_WSL_HOST_PRIVATE_KEY" -o StrictHostKeyChecking=no "$AZURE_WSL_USER@$HOST" '"C:\Program Files\WSL\wsl.exe"' -d ibwsl cat /etc/os-release)
WSL_ID=$(echo "$OS_RELEASE" | grep "^ID=" | cut -d '=' -f2  | tr -d '"')
WSL_VERSION_ID=$(echo "$OS_RELEASE" | grep "^VERSION_ID=" | cut -d '=' -f2  | tr -d '"')

if [ ! "$DISTRO_CODE" = "$WSL_ID-$WSL_VERSION_ID" ]; then
    redprint "wsl os-release ($WSL_ID-$WSL_VERSION_ID) is not the same as the test runner os-release ($DISTRO_CODE)"
    exit 1
fi

exit 0
