#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/aws.sh
source /usr/libexec/tests/osbuild-composer/api/common/common.sh
source /usr/libexec/tests/osbuild-composer/api/common/s3.sh

# Check that needed variables are set to access AWS.
function checkEnv() {
    printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null

    if [ "${IMAGE_TYPE}" = "${IMAGE_TYPE_VSPHERE}" ]; then
        printenv GOVMOMI_USERNAME GOVMOMI_PASSWORD GOVMOMI_URL GOVMOMI_CLUSTER GOVC_DATACENTER GOVMOMI_DATASTORE GOVMOMI_FOLDER GOVMOMI_NETWORK  > /dev/null
    fi
}

# Global var for ostree ref
OSTREE_REF="test/rhel/8/edge"

function cleanup() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # extract filename component from URL
  local S3_FILENAME
  S3_FILENAME=$(echo "${S3_URL}" | grep -oP '(?<=/)[^/]+(?=\?)')

  # prepend bucket
  local S3_URI
  S3_URI="s3://${AWS_BUCKET}/${S3_FILENAME}"

  # since this function can be called at any time, ensure that we don't expand unbound variables
  AWS_CMD="${AWS_CMD:-}"

  if [ -n "$AWS_CMD" ]; then
    $AWS_CMD s3 rm "${S3_URI}"
  fi
}

function createReqFile() {
    case ${IMAGE_TYPE} in
        "$IMAGE_TYPE_EDGE_COMMIT"|"$IMAGE_TYPE_EDGE_CONTAINER"|"$IMAGE_TYPE_EDGE_INSTALLER"|"$IMAGE_TYPE_IMAGE_INSTALLER")
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
  echo "$S3_URL" | grep -F "$AWS_REGION" -
}


# verify edge commit content
function verifyEdgeCommit() {
    filename="$1"
    greenprint "Verifying contents of ${filename}"

    # extract tarball and save file list to artifacts directroy
    local COMMIT_DIR
    COMMIT_DIR="${WORKDIR}/edge-commit"
    mkdir -p "${COMMIT_DIR}"
    tar xvf "${filename}" -C "${COMMIT_DIR}" > "${ARTIFACTS}/edge-commit-filelist.txt"

    # Verify that the commit contains the ref we defined in the request
    sudo dnf install -y ostree
    local COMMIT_REF
    COMMIT_REF=$(ostree refs --repo "${COMMIT_DIR}/repo")
    if [[ "${COMMIT_REF}" !=  "${OSTREE_REF}" ]]; then
        echo "Commit ref in archive does not match request 😠"
        exit 1
    fi

    local TAR_COMMIT_ID
    TAR_COMMIT_ID=$(ostree rev-parse --repo "${COMMIT_DIR}/repo" "${OSTREE_REF}")
    API_COMMIT_ID_V2=$(curl \
        --silent \
        --show-error \
        --cacert /etc/osbuild-composer/ca-crt.pem \
        --key /etc/osbuild-composer/client-key.pem \
        --cert /etc/osbuild-composer/client-crt.pem \
        https://localhost/api/image-builder-composer/v2/composes/"$COMPOSE_ID"/metadata | jq -r '.ostree_commit')

    if [[ "${API_COMMIT_ID_V2}" != "${TAR_COMMIT_ID}" ]]; then
        echo "Commit ID returned from API does not match Commit ID in archive 😠"
        exit 1
    fi

}

# Verify image blobs from s3
function verifyDisk() {
    filename="$1"
    greenprint "Verifying contents of ${filename}"

    infofile="${filename}-info.json"
    sudo /usr/libexec/osbuild-composer-test/image-info "${filename}" | tee "${infofile}" > /dev/null

    # save image info to artifacts
    cp -v "${infofile}" "${ARTIFACTS}/image-info.json"

    # check compose request users in passwd
    if ! jq .passwd "${infofile}" | grep -q "user1"; then
        greenprint "❌ user1 not found in passwd file"
        exit 1
    fi
    if ! jq .passwd "${infofile}" | grep -q "user2"; then
        greenprint "❌ user2 not found in passwd file"
        exit 1
    fi
    # check packages for postgresql
    if ! jq .packages "${infofile}" | grep -q "postgresql"; then
        greenprint "❌ postgresql not found in packages"
        exit 1
    fi

    greenprint "✅ ${filename} image info verified"
}


# Verify s3 blobs
function verify() {
    local S3_URL
    S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
    greenprint "Verifying S3 object at ${S3_URL}"

    # Tag the resource as a test file
    local S3_FILENAME
    S3_FILENAME=$(echo "${S3_URL}" | grep -oP '(?<=/)[^/]+(?=\?)')

    # tag the object, also verifying that it exists in the bucket as expected
    $AWS_CMD s3api put-object-tagging \
        --bucket "${AWS_BUCKET}" \
        --key "${S3_FILENAME}" \
        --tagging '{"TagSet": [{ "Key": "gitlab-ci-test", "Value": "true" }]}'

    greenprint "✅ Successfully tagged S3 object"

    # Download the object using the Presigned URL and inspect
    case ${IMAGE_TYPE} in
        "$IMAGE_TYPE_EDGE_COMMIT")
            curl "${S3_URL}" --output "${WORKDIR}/edge-commit.tar"
            verifyEdgeCommit "${WORKDIR}/edge-commit.tar"
            ;;
        "${IMAGE_TYPE_GUEST}")
            curl "${S3_URL}" --output "${WORKDIR}/disk.qcow2"
            verifyDisk "${WORKDIR}/disk.qcow2"
            ;;

        "${IMAGE_TYPE_VSPHERE}")
            curl "${S3_URL}" --output "${WORKDIR}/disk.vmdk"
            verifyInVSphere "${WORKDIR}/disk.vmdk"
            ;;
        *)
            greenprint "No validation method for image type ${IMAGE_TYPE}"
            ;;
    esac

    greenprint "✅ Successfully verified S3 object"
}
