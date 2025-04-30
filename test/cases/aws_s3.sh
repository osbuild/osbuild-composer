#!/bin/bash

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

set -euo pipefail

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
    echo "== Script execution stopped or finished - Cleaning up =="
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
AWS_S3_PROVIDER_CONFIG=${TEMPDIR}/aws.toml

# We need awscli to talk to AWS.
if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo "${CONTAINER_RUNTIME}" pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -e AWS_ACCESS_KEY_ID=${V2_AWS_ACCESS_KEY_ID} \
        -e AWS_SECRET_ACCESS_KEY=${V2_AWS_SECRET_ACCESS_KEY} \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} aws --region $AWS_REGION"
else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="aws --region $AWS_REGION"
fi
$AWS_CMD --version

# Write an AWS TOML file
tee "$AWS_S3_PROVIDER_CONFIG" > /dev/null << EOF
provider = "aws.s3"

[settings]
accessKeyID = "${V2_AWS_ACCESS_KEY_ID}"
secretAccessKey = "${V2_AWS_SECRET_ACCESS_KEY}"
bucket = "${AWS_BUCKET}"
region = "${AWS_REGION}"
key = "${TEST_ID}"
EOF

IMAGE_OBJECT_KEY="${AWS_BUCKET}/${TEST_ID}-disk.qcow2"

/usr/libexec/osbuild-composer-test/s3_test.sh "${TEST_ID}" "${AWS_S3_PROVIDER_CONFIG}" "${AWS_CMD} s3" "${IMAGE_OBJECT_KEY}"
