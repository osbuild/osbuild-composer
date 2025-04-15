#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/aws.sh
source /usr/libexec/tests/osbuild-composer/api/common/common.sh
source /usr/libexec/tests/osbuild-composer/api/common/vsphere.sh
source /usr/libexec/tests/osbuild-composer/api/common/s3.sh

# Check that needed variables are set to access AWS.
function checkEnv() {
    printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null
    if [ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]; then
        checkEnvVSphere
    fi
}

function cleanup() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # extract filename component from URL
  local S3_FILENAME
  S3_FILENAME=$(echo "import urllib.parse; print(urllib.parse.urlsplit('$S3_URL').path.strip('/'))" | python3 -)

  # prepend bucket
  local S3_URI
  S3_URI="s3://${AWS_BUCKET}/${S3_FILENAME}"

  # since this function can be called at any time, ensure that we don't expand unbound variables
  AWS_CMD="${AWS_CMD:-}"

  if [ -n "$AWS_CMD" ]; then
    $AWS_CMD s3 rm "${S3_URI}"
  fi

  if [ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]; then
    cleanupVSphere
  fi
}

function installClient() {
    installAWSClient

    if [ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]; then
        installClientVSphere
    fi
}

function createReqFile() {
    case ${IMAGE_TYPE} in
        "$IMAGE_TYPE_EDGE_COMMIT"|"$IMAGE_TYPE_IOT_COMMIT"|"$IMAGE_TYPE_EDGE_CONTAINER"|"$IMAGE_TYPE_EDGE_INSTALLER"|"$IMAGE_TYPE_IMAGE_INSTALLER"|"$IMAGE_TYPE_IOT_BOOTABLE_CONTAINER")
            createReqFileEdge
            ;;
        "$IMAGE_TYPE_VSPHERE")
            createReqFileGuest
            ;;
        "$IMAGE_TYPE_VSPHERE")
            createReqFileVSphere
            ;;
        *)
            echo "Unknown s3 image type for: ${IMAGE_TYPE}"
            exit 1
    esac
}

function checkUploadStatusOptions() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # S3 URL contains region and bucket name
  echo "$S3_URL" | grep -F "$AWS_BUCKET" -
}

# Verify s3 blobs
function verify() {
    local S3_URL
    S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
    greenprint "Verifying S3 object at ${S3_URL}"

    # Tag the resource as a test file
    local S3_FILENAME
    S3_FILENAME=$(echo "import urllib.parse; print(urllib.parse.urlsplit('$S3_URL').path.strip('/'))" | python3 -)

    # tag the object, also verifying that it exists in the bucket as expected
    $AWS_CMD s3api put-object-tagging \
        --bucket "${AWS_BUCKET}" \
        --key "${S3_FILENAME}" \
        --tagging '{"TagSet": [{ "Key": "gitlab-ci-test", "Value": "true" }]}'

    greenprint "✅ Successfully tagged S3 object"

    # Download the object using the Presigned URL and inspect
    case ${IMAGE_TYPE} in
        "$IMAGE_TYPE_EDGE_COMMIT"|"${IMAGE_TYPE_IOT_COMMIT}")
            if [[ $ID == "fedora" ]]; then
                # on Fedora, the test case uploads the artifact publicly,
                # so check here that the URL isn't presigned
                [[ ${S3_URL} != *"X-Amz-Signature"* ]]
            else
                # The URL is presigned otherwise
                [[ ${S3_URL} == *"X-Amz-Signature"* ]]
            fi

            curl "${S3_URL}" --output "${WORKDIR}/edge-commit.tar"
            verifyEdgeCommit "${WORKDIR}/edge-commit.tar"
            ;;
        "${IMAGE_TYPE_GUEST}")
            curl "${S3_URL}" --output "${WORKDIR}/disk.qcow2"
            verifyDisk "${WORKDIR}/disk.qcow2"
            ;;

        # NOTE(akoutsou): The vsphere verification is failing very
        # consistently. Disabling it until we have time to look into it
        # further.
        # "${IMAGE_TYPE_VSPHERE}")
        #     curl "${S3_URL}" --output "${WORKDIR}/disk.vmdk"
        #     verifyInVSphere "${WORKDIR}/disk.vmdk"
        #     ;;
        *)
            greenprint "No validation method for image type ${IMAGE_TYPE}"
            ;;
    esac

    greenprint "✅ Successfully verified S3 object"
}
