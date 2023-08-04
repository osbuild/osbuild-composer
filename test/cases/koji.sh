#!/bin/bash
set -euo pipefail

OSBUILD_COMPOSER_TEST_DATA=/usr/share/tests/osbuild-composer/

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh


if [ "${NIGHTLY:=false}" == "true" ]; then
    greenprint "INFO: Test not supported during nightly CI pipelines. Exiting ..."
    exit 1
fi

#
# Cloud provider / target names
#

CLOUD_PROVIDER_AWS="aws"
CLOUD_PROVIDER_GCP="gcp"
CLOUD_PROVIDER_AZURE="azure"

# define the default cloud provider to test for
CLOUD_PROVIDER="none"

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
        /usr/libexec/osbuild-composer-test/run-mock-auth-servers.sh stop
        set -eu
    }

    # install appropriate cloud environment client tool
    installClient
else
    # Source common functions
    # In the common case above, this file is sourced by 'aws.sh' / 'gcp.sh' / 'azure.sh'
    source /usr/libexec/tests/osbuild-composer/api/common/common.sh

    function cleanups() {
        greenprint "Script execution stopped or finished - Cleaning up"
        set +eu
        /usr/libexec/osbuild-composer-test/run-mock-auth-servers.sh stop
        set -eu
    }
fi

trap cleanups EXIT

# Verify that all the expected information is present in the buildinfo
function verify_buildinfo() {
    local buildinfo="${1}"
    local target_cloud="${2:-none}"

    local extra_build_metadata
    # extract the extra build metadata JSON from the output
    extra_build_metadata="$(echo "${buildinfo}" | grep -oP '(?<=Extra: ).*' | tr "'" '"')"

    # sanity check the extra build metadata
    if [ -z "${extra_build_metadata}" ]; then
        echo "Extra build metadata is empty"
        exit 1
    fi

    # extract the image archives paths from the output and keep only the filenames
    local outputs_images
    outputs_images="$(echo "${buildinfo}" |
        sed -zE 's/.*Image archives:\n((\S+\n){1,})([\w\s]+:){0,}.*/\1/g' |
        sed -E 's/.*\/(.*)/\1/g')"

    # we build one image for cloud test case and two for non-cloud test case
    if [ "${target_cloud}" == "none" ]; then
        if [[ $(echo "${outputs_images}" | wc -l) -ne 2 ]]; then
            echo "Unexpected number of images in the buildinfo"
            exit 1
        fi
    else
        if [[ $(echo "${outputs_images}" | wc -l) -ne 1 ]]; then
            echo "Unexpected number of images in the buildinfo"
            exit 1
        fi
    fi

    local images_metadata
    images_metadata="$(echo "${extra_build_metadata}" | jq -r '.typeinfo.image')"

    for image in $outputs_images; do
        local image_metadata
        image_metadata="$(echo "${images_metadata}" | jq -r ".\"${image}\"")"
        if [ "${image_metadata}" == "null" ]; then
            echo "Image metadata for '${image}' is missing"
            exit 1
        fi

        local image_arch
        image_arch="$(echo "${image_metadata}" | jq -r '.arch')"
        if [ "${image_arch}" != "${ARCH}" ]; then
            echo "Unexpected arch for '${image}'. Expected '${ARCH}', but got '${image_arch}'"
            exit 1
        fi

        local image_boot_mode
        image_boot_mode="$(echo "${image_metadata}" | jq -r '.boot_mode')"
        # for now, check just that the boot mode is a valid value
        case "${image_boot_mode}" in
            "uefi"|"legacy"|"hybrid")
                ;;
            "none"|*)
                # for now, we don't upload any images that have 'none' as boot mode, although it is a valid value
                echo "Unexpected boot mode for '${image}'. Expected 'uefi', 'legacy' or 'hybrid', but got '${image_boot_mode}'"
                exit 1
                ;;
        esac
    done
}

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh jwt

greenprint "Starting containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh start

greenprint "Adding kerberos config"
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

greenprint "Testing Koji"
koji --server=http://localhost:8080/kojihub --user=osbuild --password=osbuildpass --authtype=password hello

greenprint "Creating Koji task"
koji --server=http://localhost:8080/kojihub --user kojiadmin --password kojipass --authtype=password make-task image

# Always build the latest RHEL - that suits the koji API usecase the most.
if [[ "$DISTRO_CODE" == rhel-8* ]]; then
    DISTRO_CODE=rhel-89
elif [[ "$DISTRO_CODE" == rhel-9* ]]; then
    DISTRO_CODE=rhel-93
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

    COMPOSE_STATUS=$(compose_status "${COMPOSE_ID}")
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

greenprint "Show Koji buildinfo"
BUILDINFO_OUTPUT="$(koji --server=http://localhost:8080/kojihub buildinfo 1)"
echo "${BUILDINFO_OUTPUT}"

greenprint "Verify the buildinfo output"
verify_buildinfo "${BUILDINFO_OUTPUT}" "${CLOUD_PROVIDER}"

greenprint "Run the integration test"
sudo /usr/libexec/osbuild-composer-test/osbuild-koji-tests

greenprint "Stopping containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh stop

greenprint "Removing generated CA cert"
sudo rm \
    /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
sudo update-ca-trust
