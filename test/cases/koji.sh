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

# Koji hub URL to use for testing
KOJI_HUB_URL="http://localhost:8080/kojihub"

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
    local buildid="${1}"
    local target_cloud="${2:-none}"

    local osbuild_version
    osbuild_version="$(osbuild --version | cut -d ' ' -f 2 -)"

    local extra_build_metadata
    # extract the extra build metadata JSON from the output
    extra_build_metadata="$(koji -s "${KOJI_HUB_URL}" --noauth call --json getBuild "${buildid}" | jq -r '.extra')"

    # sanity check the extra build metadata
    if [ -z "${extra_build_metadata}" ]; then
        echo "Extra build metadata is empty"
        exit 1
    fi

    # extract the image archives paths from the output and keep only the filenames
    local outputs_images
    outputs_images="$(koji -s "${KOJI_HUB_URL}" --noauth call --json listArchives "${buildid}" | jq -r 'map(select(.btype == "image" and .type_name != "json"))')"

    # we build one image for cloud test case and two for non-cloud test case
    local outputs_images_count
    outputs_images_count="$(echo "${outputs_images}" | jq 'length')"
    if [ "${target_cloud}" == "none" ]; then
        if [ "${outputs_images_count}" -ne 2 ]; then
            echo "Unexpected number of images in the buildinfo. Want 2, got ${outputs_images_count}."
            exit 1
        fi
    else
        if [ "${outputs_images_count}" -ne 1 ]; then
            echo "Unexpected number of images in the buildinfo. Want 1, got ${outputs_images_count}."
            exit 1
        fi

        # Verify that the target results are present in the image output metadata
        local target_results
        target_results="$(echo "${outputs_images}" | jq -r '.[0].extra.image.upload_target_results')"
        local target_results_count
        target_results_count="$(echo "${target_results}" | jq 'length')"
        if [ "$target_results_count" -ne 1 ]; then
            echo "Unexpected number of target results in the buildinfo. Want 1, got ${target_results_count}."
            exit 1
        fi

        local target_result_name
        target_result_name="$(echo "${target_results}" | jq -r '.[0].name')"
        local want_target_result_name
        case ${target_cloud} in
            "$CLOUD_PROVIDER_AWS")
                want_target_result_name="org.osbuild.aws"
                ;;
            "$CLOUD_PROVIDER_GCP")
                want_target_result_name="org.osbuild.gcp"
                ;;
            "$CLOUD_PROVIDER_AZURE")
                want_target_result_name="org.osbuild.azure.image"
                ;;
            *)
                echo "Unknown cloud provider: ${CLOUD_PROVIDER}"
                exit 1
        esac
        if [ "${target_result_name}" != "${want_target_result_name}" ]; then
            echo "Unexpected target result in the buildinfo. Want '${want_target_result_name}', got '${target_result_name}'."
            exit 1
        fi
    fi

    local outputs_manifests
    outputs_manifests="$(koji -s "${KOJI_HUB_URL}" --noauth call --json listArchives "${buildid}" | jq -r 'map(select(.btype == "image" and .type_name == "json" and (.filename | contains(".manifest."))))')"
    local outputs_manifests_count
    outputs_manifests_count="$(echo "${outputs_manifests}" | jq 'length')"
    if [ "${outputs_manifests_count}" -ne "${outputs_images_count}" ]; then
        echo "Mismatch between the number of image archives and image manifests in the buildinfo"
        exit 1
    fi

    local outputs_sboms
    outputs_sboms="$(koji -s "${KOJI_HUB_URL}" --noauth call --json listArchives "${buildid}" | jq -r 'map(select(.btype == "image" and .type_name == "json" and (.filename | contains(".spdx."))))')"
    local outputs_sboms_count
    outputs_sboms_count="$(echo "${outputs_sboms}" | jq 'length')"
    # there should be two SPDX files for each image, one for the image payload and one for the buildroot
    if [ "${outputs_sboms_count}" -ne $((outputs_images_count * 2)) ]; then
        echo "Mismatch between the number of image archives and SPDX SBOM files in the buildinfo"
        exit 1
    fi

    local outputs_logs
    outputs_logs="$(koji -s "${KOJI_HUB_URL}" --noauth call --json getBuildLogs "${buildid}" | jq -r 'map(select(.name != "cg_import.log"))')"
    local outputs_logs_count
    outputs_logs_count="$(echo "${outputs_logs}" | jq 'length')"
    if [ "${outputs_logs_count}" -ne "${outputs_images_count}" ]; then
        echo "Mismatch between the number of image archives and image logs in the buildinfo"
        exit 1
    fi

    local build_extra_md_image
    build_extra_md_image="$(echo "${extra_build_metadata}" | jq -r '.typeinfo.image')"

    for image_idx in $(seq 0 $((outputs_images_count - 1))); do
        local image
        image="$(echo "${outputs_images}" | jq -r ".[${image_idx}]")"

        local image_filename
        image_filename="$(echo "${image}" | jq -r '.filename')"

        local image_metadata_build
        image_metadata_build="$(echo "${build_extra_md_image}" | jq -r ".\"${image_filename}\"")"
        if [ "${image_metadata_build}" == "null" ]; then
            echo "Image metadata for '${image_filename}' is missing"
            exit 1
        fi

        local image_arch
        image_arch="$(echo "${image_metadata_build}" | jq -r '.arch')"
        if [ "${image_arch}" != "${ARCH}" ]; then
            echo "Unexpected arch for '${image_filename}'. Expected '${ARCH}', but got '${image_arch}'"
            exit 1
        fi

        local image_boot_mode
        image_boot_mode="$(echo "${image_metadata_build}" | jq -r '.boot_mode')"
        # for now, check just that the boot mode is a valid value
        case "${image_boot_mode}" in
            "uefi"|"legacy"|"hybrid")
                ;;
            "none"|*)
                # for now, we don't upload any images that have 'none' as boot mode, although it is a valid value
                echo "Unexpected boot mode for '${image_filename}'. Expected 'uefi', 'legacy' or 'hybrid', but got '${image_boot_mode}'"
                exit 1
                ;;
        esac

        local image_osbuild_artifact
        image_osbuild_artifact="$(echo "${image_metadata_build}" | jq -r '.osbuild_artifact')"
        if [ "${image_osbuild_artifact}" == "null" ]; then
            echo "Image osbuild artifact information for '${image_filename}' is missing"
            exit 1
        fi

        local image_osbuild_version
        image_osbuild_version="$(echo "${image_metadata_build}" | jq -r '.osbuild_version')"
        if [ "${image_osbuild_version}" != "${osbuild_version}" ]; then
            echo "Unexpected osbuild version for '${image_filename}'. Expected '${osbuild_version}', but got '${image_osbuild_version}'"
            exit 1
        fi

        local image_metadata_archive
        image_metadata_archive="$(echo "${image}" | jq -r '.extra.image')"
        if [ "${image_metadata_build}" != "${image_metadata_archive}" ]; then
            echo "Image extra metadata for '${image_filename}' in the build metadata and in the archive metadata differ"
            exit 1
        fi
    done

    local build_extra_md_manifest
    build_extra_md_manifest="$(echo "${extra_build_metadata}" | jq -r '.osbuild_manifest')"

    for manifest_idx in $(seq 0 $((outputs_manifests_count - 1))); do
        local manifest
        manifest="$(echo "${outputs_manifests}" | jq -r ".[${manifest_idx}]")"

        local manifest_filename
        manifest_filename="$(echo "${manifest}" | jq -r '.filename')"

        local manifest_metadata_build
        manifest_metadata_build="$(echo "${build_extra_md_manifest}" | jq -r ".\"${manifest_filename}\"")"
        if [ "${manifest_metadata_build}" == "null" ]; then
            echo "Manifest metadata for '${manifest_filename}' is missing"
            exit 1
        fi

        local manifest_arch
        manifest_arch="$(echo "${manifest_metadata_build}" | jq -r '.arch')"
        if [ "${image_arch}" != "${ARCH}" ]; then
            echo "Unexpected arch for '${manifest_filename}'. Expected '${ARCH}', but got '${manifest_arch}'"
            exit 1
        fi

        local manifest_info
        manifest_info="$(echo "${manifest_metadata_build}" | jq -r '.info')"
        if [ "${manifest_info}" == "null" ]; then
            echo "Manifest info for '${manifest_filename}' is missing"
            exit 1
        fi

        if [ "$(echo "${manifest_info}" | jq -r '.osbuild_composer_version')" == "null" ]; then
            echo "Manifest info for '${manifest_filename}' is missing osbuild-composer version"
            exit 1
        fi

        # check osbuild/images version info in the metadata
        local osbuild_composer_deps
        osbuild_composer_deps="$(echo "${manifest_info}" | jq -r '.osbuild_composer_deps')"
        if [ "$(echo "${osbuild_composer_deps}" | jq 'length')" -ne 1 ]; then
            echo "Manifest info for '${manifest_filename}' has unexpected number of osbuild-composer dependencies. \
                Expected 1, got '$(echo "${osbuild_composer_deps}" | jq 'length')'"
            exit 1
        fi
        if [ "$(echo "${osbuild_composer_deps}" | jq -r '.[0].path')" != "github.com/osbuild/images" ]; then
            echo "Manifest info for '${manifest_filename}' has unexpected osbuild-composer dependency path. \
                Expected 'github.com/osbuild/images', got '$(echo "${osbuild_composer_deps}" | jq -r '.[0].path')'"
            exit 1
        fi
        if [ "$(echo "${osbuild_composer_deps}" | jq -r '.[0].version')" == "null" ]; then
            echo "Manifest info for '${manifest_filename}' has missing 'github.com/osbuild/images' dependency version"
            exit 1
        fi

        local manifest_metadata_archive
        manifest_metadata_archive="$(echo "${manifest}" | jq -r '.extra.image')"
        if [ "${manifest_metadata_build}" != "${manifest_metadata_archive}" ]; then
            echo "Manifest extra metadata for '${manifest_filename}' in the build metadata and in the archive metadata differ"
            exit 1
        fi
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
koji --server="${KOJI_HUB_URL}" --user=osbuild --password=osbuildpass --authtype=password hello

greenprint "Creating Koji task"
koji --server="${KOJI_HUB_URL}" --user kojiadmin --password kojipass --authtype=password make-task image

# Always build the latest RHEL - that suits the koji API usecase the most.
if [[ "$DISTRO_CODE" == rhel-8* ]]; then
    DISTRO_CODE=rhel-8.10
elif [[ "$DISTRO_CODE" == rhel-9* ]]; then
    DISTRO_CODE=rhel-9.5
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
koji --server="${KOJI_HUB_URL}" taskinfo 1

greenprint "Show Koji buildinfo"
koji --server="${KOJI_HUB_URL}" buildinfo 1

greenprint "Show Koji raw buildinfo"
koji --server="${KOJI_HUB_URL}" --noauth call --json getBuild 1

greenprint "Show Koji build archives"
koji --server="${KOJI_HUB_URL}" --noauth call --json listArchives 1

greenprint "Show Koji build logs"
koji --server="${KOJI_HUB_URL}" --noauth call --json getBuildLogs 1

greenprint "Verify the Koji build info and metadata"
verify_buildinfo 1 "${CLOUD_PROVIDER}"

greenprint "Run the integration test"
sudo /usr/libexec/osbuild-composer-test/osbuild-koji-tests

greenprint "Stopping containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh stop

greenprint "Removing generated CA cert"
sudo rm \
    /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
sudo update-ca-trust
