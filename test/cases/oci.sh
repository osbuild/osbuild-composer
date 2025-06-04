#!/bin/bash

#
# Test osbuild-composer 'upload to oci' functionality. To do so, create
# and push a blueprint with composer cli. Then, create an instance in
# oci from the uploaded image. Finally, verify that the instance is
# running.
#

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

if type -p podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME=docker
else
    echo No container runtime found, install podman or docker.
    exit 2
fi

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
TEMPDIR=$(mktemp -d)

# Set up temporary files.
OCI_UPLOAD=${TEMPDIR}/oci.toml
OCI_CONFIG=${TEMPDIR}/oci-config
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${TEST_ID}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${TEST_ID}.json
OCI_IMAGE_DATA=${TEMPDIR}/oci-image-data-${TEST_ID}.json
SSH_DATA_DIR=$(tools/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa

OCI_USER=$(jq -r '.user' "$OCI_SECRETS")
OCI_TENANCY=$(jq -r '.tenancy' "$OCI_SECRETS")
OCI_REGION=$(jq -r '.region' "$OCI_SECRETS")
OCI_FINGERPRINT=$(jq -r '.fingerprint' "$OCI_SECRETS")
OCI_BUCKET=$(jq -r '.bucket' "$OCI_SECRETS")
OCI_NAMESPACE=$(jq -r '.namespace' "$OCI_SECRETS")
OCI_COMPARTMENT=$(jq -r '.compartment' "$OCI_SECRETS")
OCI_AUTH_TOKEN=$(jq -r '.auth_token' "$OCI_SECRETS")
OCI_SUBNET=$(jq -r '.subnet' "$OCI_SECRETS")
OCI_PRIV_KEY=$(cat "$OCI_PRIVATE_KEY")

function cleanup() {
    set +eu
    greenprint "🧼 Cleaning up"
    $OCI_CMD compute instance terminate --instance-id "${INSTANCE_ID}" --force
    $OCI_CMD compute image delete --image-id "${OCI_IMAGE_ID}" --force
    sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
    sudo rm -rf "$TEMPDIR"
    sudo pkill -P "${WORKER_JOURNAL_PID}"
    set -eu
}
trap cleanup EXIT

# copy private key to what oci considers a valid path
cp -p "$OCI_PRIVATE_KEY" "$TEMPDIR/priv_key.pem"
tee "$OCI_CONFIG" > /dev/null << EOF
[DEFAULT]
user=${OCI_USER}
fingerprint=${OCI_FINGERPRINT}
key_file=${TEMPDIR}/priv_key.pem
tenancy=${OCI_TENANCY}
region=${OCI_REGION}
EOF

if ! hash oci 2>/dev/null; then
    echo "Using 'oci' from a container"
    sudo "${CONTAINER_RUNTIME}" pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # OCI_CLI_AUTH
    OCI_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        --net=host \
        -e OCI_AUTH_TOKEN=${OCI_AUTH_TOKEN} \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        -v ${SSH_DATA_DIR}:${SSH_DATA_DIR}:Z \
        -v ${OCI_PRIVATE_KEY}:${OCI_PRIVATE_KEY}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} /root/bin/oci --config-file $OCI_CONFIG --region $OCI_REGION --output json"
else
    echo "Using pre-installed 'oci' from the system"
    OCI_CMD="oci --config-file $OCI_CONFIG --region $OCI_REGION"
fi

echo -n "OCI version: "
$OCI_CMD --version
$OCI_CMD setup repair-file-permissions --file "${TEMPDIR}/priv_key.pem"
$OCI_CMD setup repair-file-permissions --file "$OCI_CONFIG"

get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-oci.log
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-oci.json
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

function get_availability_domain_by_shape {
    for ad in $($OCI_CMD iam availability-domain list -c "$OCI_COMPARTMENT" | jq -r '.data[].name');do
        if [ "$($OCI_CMD compute shape list -c "$OCI_COMPARTMENT" --availability-domain "$ad" | jq --arg SHAPE "$1" -r '.data[]|select(.shape==$SHAPE)|.shape')" == "$1" ];then
            echo "$ad"
            return
        fi
    done
    return 1
}

# Write an OCI TOML file
tee "$OCI_UPLOAD" > /dev/null << EOF
provider = "oci"

[settings]
user = "${OCI_USER}"
private_key = '''
${OCI_PRIV_KEY}
'''
bucket = "${OCI_BUCKET}"
region = "${OCI_REGION}"
fingerprint = "${OCI_FINGERPRINT}"
compartment = "${OCI_COMPARTMENT}"
namespace = "${OCI_NAMESPACE}"
tenancy = "${OCI_TENANCY}"
EOF

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "bash"
description = "A base system"
version = "0.0.1"
EOF

greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve bash

WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!

greenprint "🚀 Starting compose"
sudo composer-cli --json compose start bash oci "$TEST_ID" "$OCI_UPLOAD" | tee "$COMPOSE_START"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")
    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi
    sleep 30
done

greenprint "💬 Getting compose log and metadata"
get_compose_log "$COMPOSE_ID"
get_compose_metadata "$COMPOSE_ID"

# Kill the journal monitor immediately
sudo pkill -P ${WORKER_JOURNAL_PID}

if [[ $COMPOSE_STATUS != FINISHED ]]; then
    redprint "Something went wrong with the compose. 😢"
    exit 1
fi

# Find the image that we made in OCI.
greenprint "🔍 Searching for created OCI"
$OCI_CMD compute image list --compartment-id "${OCI_COMPARTMENT}" --display-name "${TEST_ID}"
RET=$($OCI_CMD compute image list --compartment-id "${OCI_COMPARTMENT}" --display-name "${TEST_ID}")
echo "$RET"
echo "$RET" | jq '.data[0]' | tee "$OCI_IMAGE_DATA" > /dev/null
OCI_IMAGE_ID=$(jq -r '.id' "$OCI_IMAGE_DATA")
greenprint  "🔍 Found $OCI_IMAGE_ID, searching shape availability"

SHAPE="VM.Standard.E4.Flex"
OCI_AVAILABILITY_DOMAIN="$(get_availability_domain_by_shape "$SHAPE")"

# Build instance in OCI with our image.
greenprint "👷 Building instance in OCI"

INSTANCE=$($OCI_CMD compute instance launch  \
                    --shape $SHAPE \
                    --shape-config '{"memoryInGBs": 2.0, "ocpus": 1.0}' \
                    -c "${OCI_COMPARTMENT}" \
                    --availability-domain "${OCI_AVAILABILITY_DOMAIN}" \
                    --subnet-id "${OCI_SUBNET}" \
                    --image-id "${OCI_IMAGE_ID}" \
                    --freeform-tags "{\"Name\": \"${TEST_ID}\", \"gitlab-ci-test\": \"true\"}" \
                    --user-data-file "${SSH_DATA_DIR}/user-data")
echo "Attempted to launch instance: $INSTANCE"
INSTANCE_ID="$(echo "$INSTANCE" | jq -r '.data.id')"

greenprint "⏱ Waiting for OCI instance to be marked as running"
while true; do
    INSTANCE=$($OCI_CMD compute instance get --instance-id "$INSTANCE_ID")
    if [[ $(echo "$INSTANCE" | jq -r '.data["lifecycle-state"]') == RUNNING ]]; then
        break
    fi
    sleep 10
done

# Get data about the instance we built.
PUBLIC_IP=$($OCI_CMD compute instance list-vnics --instance-id "$INSTANCE_ID" |  jq -r '.data[0]["public-ip"]')

greenprint "⏱ Waiting for OCI instance to respond to ssh"
for (( i=0 ; i<30; i++ )); do
    if ssh-keyscan "$PUBLIC_IP" > /dev/null 2>&1; then
        echo "SSH is up!"
        ssh-keyscan "$PUBLIC_IP" | sudo tee -a /root/.ssh/known_hosts
        break
    fi

    # ssh-keyscan has a 5 second timeout by default, so the pause per loop
    # is 10 seconds when you include the following `sleep`.
    echo "Retrying in 5 seconds..."
    sleep 5
done

ssh -o StrictHostKeyChecking=no -i "$SSH_KEY" redhat@"$PUBLIC_IP" uname -a
greenprint "💚 Success"
exit 0
