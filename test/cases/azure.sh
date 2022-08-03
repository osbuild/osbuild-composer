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
BRANCH_NAME="${CI_COMMIT_BRANCH:-local}"
BUILD_ID="${CI_BUILD_ID:-$(uuidgen)}"
HYPER_V_GEN="${HYPER_V_GEN:-V1}"

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

TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

# Terraform needs azure-cli to talk to Azure.
if ! hash az; then
    echo "Using 'azure-cli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # directory mounted to the container, in which azure-cli stores the credentials after logging in
    AZURE_CMD_CREDS_DIR="${TEMPDIR}/azure-cli_credentials"
    mkdir "${AZURE_CMD_CREDS_DIR}"

    AZURE_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -v ${AZURE_CMD_CREDS_DIR}:/root/.azure:Z \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} az"
    alias az='${AZURE_CMD}'
else
    echo "Using pre-installed 'azure-cli' from the system"
fi
az version

# We need terraform to provision the vm in azure and then destroy it
if [ "$ID" == "rhel" ] || [ "$ID" == "centos" ]
then
    release="RHEL"
elif [ "$ID" == "fedora" ]
then
    release="fedora"
else
    echo "Test is not running on neither Fedora, RHEL or CentOS, terminating!"
    exit 1
fi
sudo dnf config-manager --add-repo https://rpm.releases.hashicorp.com/$release/hashicorp.repo
# set $releasever to 8 when running on RHEL-9 because there is no hashicorp repo for it yet
if [[ $ID == rhel || $ID == centos ]] && [[ ${VERSION_ID%.*} == 9 ]]; then
    # shellcheck disable=SC2016
    sudo sed -i 's/$releasever/8/g' /etc/yum.repos.d/hashicorp.repo
fi
sudo dnf install -y terraform

ARCH=$(uname -m)
TEST_ID="$DISTRO_CODE-$ARCH-$BRANCH_NAME-$BUILD_ID"
IMAGE_KEY=image-${TEST_ID}

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Set up temporary files.
AZURE_CONFIG=${TEMPDIR}/azure.toml
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# Check for the smoke test file on the Azure instance that we start.
smoke_test_check () {
    SMOKE_TEST=$(sudo ssh -i key.rsa redhat@"${1}" -o StrictHostKeyChecking=no 'cat /etc/smoke-test.txt')
    if [[ $SMOKE_TEST == smoke-test ]]; then
        echo 1
    else
        echo 0
    fi
}

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

is_weldr_client_installed () {
    if rpm --quiet -q weldr-client; then
        echo true
    else
        echo false
    fi
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
# Stop watching the worker journal when exiting.
trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

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

# Kill the journal monitor immediately and remove the trap
sudo pkill -P ${WORKER_JOURNAL_PID}
trap - EXIT

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi

# Set up necessary variables for terraform
export TF_VAR_RESOURCE_GROUP="$AZURE_RESOURCE_GROUP"
export TF_VAR_STORAGE_ACCOUNT="$AZURE_STORAGE_ACCOUNT"
export TF_VAR_CONTAINER_NAME="$AZURE_CONTAINER_NAME"
export TF_VAR_BLOB_NAME="$IMAGE_KEY".vhd
export TF_VAR_TEST_ID="$TEST_ID"
# https://registry.terraform.io/providers/hashicorp/azurerm/latest/docs/resources/image#argument-reference
export TF_VAR_HYPER_V_GEN="${HYPER_V_GEN}"
export BLOB_URL="https://$AZURE_STORAGE_ACCOUNT.blob.core.windows.net/$AZURE_CONTAINER_NAME/$IMAGE_KEY.vhd"
export ARM_CLIENT_ID="$V2_AZURE_CLIENT_ID" > /dev/null
export ARM_CLIENT_SECRET="$V2_AZURE_CLIENT_SECRET" > /dev/null
export ARM_SUBSCRIPTION_ID="$AZURE_SUBSCRIPTION_ID" > /dev/null
export ARM_TENANT_ID="$AZURE_TENANT_ID" > /dev/null

SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)

# Copy terraform main file and cloud-init to current working directory
cp /usr/share/tests/osbuild-composer/azure/main.tf .
cp "${SSH_DATA_DIR}"/user-data .

# Initialize terraform
terraform init

# Import the uploaded page blob to terraform
terraform import azurerm_storage_blob.testBlob "$BLOB_URL"

# Apply the configuration
terraform apply -auto-approve

PUBLIC_IP=$(terraform output -raw public_IP)
terraform output -raw tls_private_key > key.rsa
chmod 400 key.rsa

# Check for our smoke test file.
greenprint "🛃 Checking for smoke test file"
for _ in {0..10}; do
    RESULTS="$(smoke_test_check "$PUBLIC_IP")"
    if [[ $RESULTS == 1 ]]; then
        echo "Smoke test passed! 🥳"
        break
    fi
    echo "Machine is not ready yet, retrying connection."
    sleep 5
done

# Clean up resources in Azure
terraform destroy -auto-approve

# Also delete the compose so we don't run out of disk space
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null

# Use the return code of the smoke test to determine if we passed or failed.
if [[ $RESULTS == 1 ]]; then
    greenprint "💚 Success with HyperV ${HYPER_V_GEN}"
    exit 0
elif [[ $RESULTS != 1 ]]; then
    greenprint "❌ Failed ${HYPER_V_GEN}"
    exit 1
fi

exit 0

