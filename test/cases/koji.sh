#!/bin/bash
set -euo pipefail

OSBUILD_COMPOSER_TEST_DATA=/usr/share/tests/osbuild-composer/

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

if [ "${NIGHTLY:=false}" == "true" ]; then
    greenprint "INFO: Test not supported during nightly CI pipelines. Exiting ..."
    exit 0
fi

#
# Cloud provider / target names
#

CLOUD_PROVIDER_AWS="aws"
CLOUD_PROVIDER_GCP="gcp"
CLOUD_PROVIDER_AZURE="azure"

#
# Test types
#
# Tests Koji compose via cloudapi
TEST_TYPE_CLOUDAPI="cloudapi"
# Tests Koji compose via cloudapi with upload to a cloud target
TEST_TYPE_CLOUD_UPLOAD="cloud-upload"

# test Koji compose via cloudapi without upload to cloud by default
TEST_TYPE="${1:-$TEST_TYPE_CLOUDAPI}"

#
# Cloud upload - check environment and prepare it
#
if [[ "$TEST_TYPE" == "$TEST_TYPE_CLOUD_UPLOAD" ]]; then
    if [[ $# -ne 3 ]]; then
        echo "Usage: $0 cloud-upload TARGET IMAGE_TYPE"
        exit 1
    fi

    CLOUD_PROVIDER="${2}"
    IMAGE_TYPE="${3}"

    greenprint "Using Cloud Provider / Target ${CLOUD_PROVIDER} for Image Type ${IMAGE_TYPE}"

    # Load a correct test runner.
    # Each one must define following methods:
    # - checkEnv
    # - cleanup
    # - installClient
    case $CLOUD_PROVIDER in
        "$CLOUD_PROVIDER_AWS")
            source /usr/libexec/tests/osbuild-composer/api/aws.sh
            ;;
        "$CLOUD_PROVIDER_GCP")
            source /usr/libexec/tests/osbuild-composer/api/gcp.sh
            ;;
        "$CLOUD_PROVIDER_AZURE")
            source /usr/libexec/tests/osbuild-composer/api/azure.sh
            ;;
        *)
            echo "Unknown cloud provider: ${CLOUD_PROVIDER}"
            exit 1
    esac

    # Verify that this script is running in the right environment.
    checkEnv

    # Container image used for cloud provider CLI tools
    export CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

    if which podman 2>/dev/null >&2; then
        export CONTAINER_RUNTIME=podman
    elif which docker 2>/dev/null >&2; then
        export CONTAINER_RUNTIME=docker
    else
        echo No container runtime found, install podman or docker.
        exit 1
    fi

    WORKDIR=$(mktemp -d)
    export WORKDIR
    function cleanups() {
        greenprint "Script execution stopped or finished - Cleaning up"
        set +eu
        cleanup
        sudo rm -rf "$WORKDIR"
        set -eu
    }
    trap cleanups EXIT

    # install appropriate cloud environment client tool
    installClient
fi

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

greenprint "Starting containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh start

greenprint "Adding kerberos config"
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/osbuild-composer/client.keytab
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/osbuild-worker/client.keytab
sudo cp \
    "${OSBUILD_COMPOSER_TEST_DATA}"/kerberos/krb5-local.conf \
    /etc/krb5.conf.d/local

greenprint "Adding the testsuite's CA cert to the system trust store"
sudo cp \
    /etc/osbuild-composer/ca-crt.pem \
    /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
sudo update-ca-trust

greenprint "Restarting composer to pick up new config"
sudo systemctl restart osbuild-composer
sudo systemctl restart osbuild-worker\@1

greenprint "Testing Koji"
koji --server=http://localhost:8080/kojihub --user=osbuild --password=osbuildpass --authtype=password hello

greenprint "Creating Koji task"
koji --server=http://localhost:8080/kojihub --user kojiadmin --password kojipass --authtype=password make-task image

# Always build the latest RHEL - that suits the koji API usecase the most.
if [[ "$DISTRO_CODE" == rhel-8* ]]; then
    DISTRO_CODE=rhel-87
elif [[ "$DISTRO_CODE" == rhel-9* ]]; then
    DISTRO_CODE=rhel-91
fi

case ${TEST_TYPE} in
    "$TEST_TYPE_CLOUDAPI")
        greenprint "Pushing compose to Koji (/api/image-builder-comoser/v2/)"
        COMPOSE_ID="$(sudo /usr/libexec/osbuild-composer-test/koji-compose.py "${DISTRO_CODE}" "${ARCH}")"
        ;;
    "$TEST_TYPE_CLOUD_UPLOAD")
        greenprint "Pushing compose to Koji (/api/image-builder-comoser/v2/) with cloud upload target"
        COMPOSE_ID="$(sudo -E /usr/libexec/osbuild-composer-test/koji-compose.py "${DISTRO_CODE}" "${ARCH}" "${CLOUD_PROVIDER}" "${IMAGE_TYPE}")"
        ;;
    *)
        echo "Unknown test type: ${TEST_TYPE}"
        exit 1
esac

if [[ "$TEST_TYPE" == "$TEST_TYPE_CLOUD_UPLOAD" ]]; then
    greenprint "Verify that image was uploaded to the cloud provider"

    COMPOSE_STATUS=$(curl \
        --silent \
        --show-error \
        --cacert /etc/osbuild-composer/ca-crt.pem \
        --key /etc/osbuild-composer/client-key.pem \
        --cert /etc/osbuild-composer/client-crt.pem \
        "https://localhost/api/image-builder-composer/v2/composes/${COMPOSE_ID}")
    UPLOAD_OPTIONS=$(echo "${COMPOSE_STATUS}" | jq -r '.image_status.upload_status.options')

    # Authenticate with the appropriate cloud
    cloud_login

    case $CLOUD_PROVIDER in
        "$CLOUD_PROVIDER_AWS")
            AMI_IMAGE_ID=$(echo "${UPLOAD_OPTIONS}" | jq -r '.ami')
            $AWS_CMD ec2 describe-images \
                --owners self \
                --filters Name=image-id,Values="${AMI_IMAGE_ID}" \
                > "${WORKDIR}/ami.json"
            # extract the snapshot ID for the purpose of cleanup
            AWS_SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' "$WORKDIR/ami.json")
            export AWS_SNAPSHOT_ID
            if [[ $(jq '.Images | length' "${WORKDIR}/ami.json") -ne 1 ]]; then
                echo "${AMI_IMAGE_ID} image not found in AWS"
                exit 1
            fi
            ;;
        "$CLOUD_PROVIDER_GCP")
            GCP_IMAGE_NAME=$(echo "${UPLOAD_OPTIONS}" | jq -r '.image_name')
            # The command exits with non-zero value if the image does not exist
            $GCP_CMD compute images describe "${GCP_IMAGE_NAME}"
            ;;
        "$CLOUD_PROVIDER_AZURE")
            AZURE_IMAGE_NAME=$(echo "${UPLOAD_OPTIONS}" | jq -r '.image_name')
            # The command exits with non-zero value if the image does not exist
            $AZURE_CMD image show --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}"
            ;;
        *)
            echo "Unknown cloud provider: ${CLOUD_PROVIDER}"
            exit 1
    esac

    # if we got here, the image must have been found
    greenprint "Image was SUCCESSFULLY found in the respective cloud provider environment!"
fi

greenprint "Show Koji task"
koji --server=http://localhost:8080/kojihub taskinfo 1
koji --server=http://localhost:8080/kojihub buildinfo 1

greenprint "Run the integration test"
sudo /usr/libexec/osbuild-composer-test/osbuild-koji-tests

greenprint "Stopping containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh stop

greenprint "Removing generated CA cert"
sudo rm \
    /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
sudo update-ca-trust
