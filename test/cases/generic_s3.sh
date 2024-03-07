#!/bin/bash

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

set -euo pipefail

# Container images for MinIO Server and Client
CONTAINER_MINIO_CLIENT="quay.io/minio/mc:latest"
CONTAINER_MINIO_SERVER="quay.io/minio/minio:latest"

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
# note: moved here to avoid undefined variable in cleanup()
MINIO_CONTAINER_NAME="minio-server"
function cleanup() {
    echo "== Script execution stopped or finished - Cleaning up =="
    sudo rm -rf "$TEMPDIR"

    # Kill the MinIO server once we're done
    ${CONTAINER_RUNTIME} kill ${MINIO_CONTAINER_NAME}
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

# Set up temporary files.
MINIO_CONFIG_DIR=${TEMPDIR}/minio-config
MINIO_PROVIDER_CONFIG=${TEMPDIR}/minio.toml

# We need MinIO Client to talk to the MinIO Server.
if ! hash mc; then
    echo "Using 'mc' from a container"
    sudo "${CONTAINER_RUNTIME}" pull ${CONTAINER_MINIO_CLIENT}

    MC_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -v ${MINIO_CONFIG_DIR}:${MINIO_CONFIG_DIR}:Z \
        --network=host \
        ${CONTAINER_MINIO_CLIENT} --config-dir=${MINIO_CONFIG_DIR}"
else
    echo "Using pre-installed 'mc' from the system"
    MC_CMD="mc --config-dir=${MINIO_CONFIG_DIR}"
fi
mkdir "${MINIO_CONFIG_DIR}"
$MC_CMD --version

MINIO_ENDPOINT="http://localhost:9000"
MINIO_ROOT_USER="X29DU5Q6C5NKDQ8PLGVT"
MINIO_ROOT_PASSWORD=$(date +%s | sha256sum | base64 | head -c 32 ; echo)
MINIO_SERVER_ALIAS=local
MINIO_BUCKET="ci-test"
MINIO_REGION="us-east-1"
# Write an AWS TOML file
tee "$MINIO_PROVIDER_CONFIG" > /dev/null << EOF
provider = "generic.s3"

[settings]
endpoint = "${MINIO_ENDPOINT}"
accessKeyID = "${MINIO_ROOT_USER}"
secretAccessKey = "${MINIO_ROOT_PASSWORD}"
bucket = "${MINIO_BUCKET}"
region = "${MINIO_REGION}"
key = "${TEST_ID}"
EOF

# Start the MinIO Server
${CONTAINER_RUNTIME} run --rm -d \
    --name ${MINIO_CONTAINER_NAME} \
    -p 9000:9000 \
    -e MINIO_BROWSER=off \
    -e MINIO_ROOT_USER="${MINIO_ROOT_USER}" \
    -e MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD}" \
    ${CONTAINER_MINIO_SERVER} server /data

# Configure the local server (retry until the service is up)
MINIO_CONFIGURE_RETRY=0
MINIO_CONFIGURE_MAX_RETRY=5
MINIO_RETRY_INTERVAL=15
until [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]
do
    ${MC_CMD} alias set ${MINIO_SERVER_ALIAS} ${MINIO_ENDPOINT} ${MINIO_ROOT_USER} "${MINIO_ROOT_PASSWORD}" && break
    MINIO_CONFIGURE_RETRY=$(${MINIO_CONFIGURE_RETRY} + 1)
    echo "Retrying [${MINIO_CONFIGURE_RETRY}/${MINIO_CONFIGURE_MAX_RETRY}] in ${MINIO_RETRY_INTERVAL}(s) "
    sleep ${MINIO_RETRY_INTERVAL}
done

if [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]; then
    echo "Failed to set MinIO alias after ${MINIO_CONFIGURE_MAX_RETRY} attempts!"
    exit 1
fi

# Create the bucket
${MC_CMD} mb ${MINIO_SERVER_ALIAS}/${MINIO_BUCKET}

IMAGE_OBJECT_KEY="${MINIO_SERVER_ALIAS}/${MINIO_BUCKET}/${TEST_ID}-disk.qcow2"
/usr/libexec/osbuild-composer-test/s3_test.sh "${TEST_ID}" "${MINIO_PROVIDER_CONFIG}" "${MC_CMD} ls ${IMAGE_OBJECT_KEY}" "${MC_CMD} --json share download ${IMAGE_OBJECT_KEY} | jq .share | tr -d '\"'"
