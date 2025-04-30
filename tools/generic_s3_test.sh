#!/bin/bash

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

CERTS_DIR=${1:-""}
CA_BUNDLE_FILENAME=${2:-""}

ENDPOINT_SCHEME="http"
if [ -n "${CERTS_DIR}" ]; then
    ENDPOINT_SCHEME="https"
fi

CA_BUNDLE_PATH=""
if [ -n "${CERTS_DIR}" ]; then
    if [ -n "${CA_BUNDLE_FILENAME}" ]; then
        CA_BUNDLE_PATH=$CERTS_DIR/$CA_BUNDLE_FILENAME
    else
        CA_BUNDLE_PATH="skip"
    fi
fi

set -euo pipefail

# Container images for MinIO Server
CONTAINER_MINIO_SERVER="quay.io/minio/minio:latest"
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

# Generate a string, which can be used as a predictable resource name,
# especially when running the test in CI where we may need to clean up
# resources in case the test unexpectedly fails or is canceled
CI="${CI:-false}"
CI_BUILD_ID=${CI_BUILD_ID:-$(uuidgen)}
if [[ "$CI" == true ]]; then
  # in CI, imitate GenerateCIArtifactName() from internal/test/helpers.go
  TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_BUILD_ID"
else
  # if not running in Jenkins, generate ID not relying on specific env variables
  TEST_ID=$(uuidgen);
fi

# Set up temporary files.
MINIO_PROVIDER_CONFIG=${TEMPDIR}/minio.toml
MINIO_ENDPOINT="$ENDPOINT_SCHEME://localhost:9000"
MINIO_ROOT_USER="X29DU5Q6C5NKDQ8PLGVT"
MINIO_ROOT_PASSWORD=$(date +%s | sha256sum | base64 | head -c 32 ; echo)
MINIO_BUCKET="ci-test"
MINIO_REGION="us-east-1"

# We need awscli to talk to the S3 Server.
if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo ${CONTAINER_RUNTIME} pull "${CONTAINER_IMAGE_CLOUD_TOOLS}"

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        --network=host \
        -e AWS_ACCESS_KEY_ID=${MINIO_ROOT_USER} \
        -e AWS_SECRET_ACCESS_KEY=${MINIO_ROOT_PASSWORD}"

    if [ -n "${CA_BUNDLE_PATH}" ] && [ "${CA_BUNDLE_PATH}" != "skip" ]; then
        AWS_CMD="${AWS_CMD} -v ${CA_BUNDLE_PATH}:${CA_BUNDLE_PATH}:z"
    fi

    AWS_CMD="${AWS_CMD} ${CONTAINER_IMAGE_CLOUD_TOOLS}"
else
    echo "Using pre-installed 'aws' from the system"
fi
AWS_CMD="${AWS_CMD} aws --region $MINIO_REGION --endpoint-url $MINIO_ENDPOINT"
if [ -n "${CA_BUNDLE_PATH}" ]; then
    if [ "${CA_BUNDLE_PATH}" == "skip" ]; then
        AWS_CMD="${AWS_CMD} --no-verify-ssl"
    else
        AWS_CMD="${AWS_CMD} --ca-bundle $CA_BUNDLE_PATH"
    fi
fi
$AWS_CMD --version
S3_CMD="${AWS_CMD} s3"

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
if [ -n "${CA_BUNDLE_PATH}" ]; then
    if [ "${CA_BUNDLE_PATH}" == "skip" ]; then
        echo "skip_ssl_verification = true"  >> "$MINIO_PROVIDER_CONFIG"
    else
        echo "ca_bundle = \"${CA_BUNDLE_PATH}\"" >> "$MINIO_PROVIDER_CONFIG"
    fi
fi

# Start the MinIO Server
MINIO_CONTAINER_NAME="minio-server"
if [ -z "${CERTS_DIR}" ]; then
    sudo ${CONTAINER_RUNTIME} run --rm -d \
        --name ${MINIO_CONTAINER_NAME} \
        -p 9000:9000 \
        -e MINIO_BROWSER=off \
        -e MINIO_ROOT_USER="${MINIO_ROOT_USER}" \
        -e MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD}" \
        ${CONTAINER_MINIO_SERVER} server /data
else
    sudo ${CONTAINER_RUNTIME} run --rm -d \
        --name ${MINIO_CONTAINER_NAME} \
        -p 9000:9000 \
        -e MINIO_BROWSER=off \
        -e MINIO_ROOT_USER="${MINIO_ROOT_USER}" \
        -e MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD}" \
        -v "${CERTS_DIR}":/root/.minio/certs:z \
        ${CONTAINER_MINIO_SERVER} server /data
fi
# Kill the server once we're done
trap 'sudo ${CONTAINER_RUNTIME} kill ${MINIO_CONTAINER_NAME}' EXIT

# Configure the local server (retry until the service is up)
MINIO_CONFIGURE_RETRY=0
MINIO_CONFIGURE_MAX_RETRY=5
MINIO_RETRY_INTERVAL=15
until [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]
do
    ${S3_CMD} ls && break
    MINIO_CONFIGURE_RETRY=$((MINIO_CONFIGURE_RETRY + 1))
	echo "Retrying [${MINIO_CONFIGURE_RETRY}/${MINIO_CONFIGURE_MAX_RETRY}] in ${MINIO_RETRY_INTERVAL}(s) "
	sleep ${MINIO_RETRY_INTERVAL}
done

if [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]; then
  echo "Failed to communicate with the MinIO server after ${MINIO_CONFIGURE_MAX_RETRY} attempts!"
  exit 1
fi

# Create the bucket
${S3_CMD} mb s3://${MINIO_BUCKET}

IMAGE_OBJECT_KEY="${MINIO_BUCKET}/${TEST_ID}-disk.qcow2"
/usr/libexec/osbuild-composer-test/s3_test.sh "${TEST_ID}" "${MINIO_PROVIDER_CONFIG}" "${S3_CMD}" "${IMAGE_OBJECT_KEY}" "${CA_BUNDLE_PATH}"
