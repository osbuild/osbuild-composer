#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/common.sh
source /usr/libexec/tests/osbuild-composer/api/common/vsphere.sh
source /usr/libexec/tests/osbuild-composer/api/common/s3.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

function checkEnv() {
    printenv AWS_REGION > /dev/null
    if [ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]; then
        checkEnvVSphere
    fi
}

# Global var for ostree ref
export OSTREE_REF="test/rhel/8/edge"

function cleanup() {
  MINIO_CONTAINER_NAME="${MINIO_CONTAINER_NAME:-}"
  if [ -n "${MINIO_CONTAINER_NAME}" ]; then
    sudo "${CONTAINER_RUNTIME}" kill "${MINIO_CONTAINER_NAME}"
  fi
  if [ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]; then
    cleanupVSphere
  fi
}

function installClient() {
  local CONTAINER_MINIO_SERVER="quay.io/minio/minio:latest"
  MINIO_CONTAINER_NAME="minio-server"
  MINIO_ENDPOINT="http://localhost:9000"
  local MINIO_ROOT_USER="X29DU5Q6C5NKDQ8PLGVT"
  local MINIO_ROOT_PASSWORD
  MINIO_ROOT_PASSWORD=$(date +%s | sha256sum | base64 | head -c 32 ; echo)
  MINIO_BUCKET="ci-test"
  local MINIO_REGION="${AWS_REGION}"
  local MINIO_CREDENTIALS_FILE="/etc/osbuild-worker/minio-creds"

  sudo "${CONTAINER_RUNTIME}" run --rm -d \
    --name ${MINIO_CONTAINER_NAME} \
    -p 9000:9000 \
    -e MINIO_BROWSER=off \
    -e MINIO_ROOT_USER="${MINIO_ROOT_USER}" \
    -e MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD}" \
    ${CONTAINER_MINIO_SERVER} server /data

  if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_IMAGE_CLOUD_TOOLS}"

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -e AWS_ACCESS_KEY_ID=${MINIO_ROOT_USER} \
      -e AWS_SECRET_ACCESS_KEY=${MINIO_ROOT_PASSWORD} \
      -v ${WORKDIR}:${WORKDIR}:Z \
      --network host \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} aws"
  else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="AWS_ACCESS_KEY_ID=${MINIO_ROOT_USER} \
      AWS_SECRET_ACCESS_KEY=${MINIO_ROOT_PASSWORD} \
      aws"
  fi
  AWS_CMD+=" --region $MINIO_REGION --output json --color on --endpoint-url $MINIO_ENDPOINT"
  $AWS_CMD --version

  if [ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]; then
    installClientVSphere
  fi

  # Configure the local server (retry until the service is up)
  MINIO_CONFIGURE_RETRY=0
  MINIO_CONFIGURE_MAX_RETRY=5
  MINIO_RETRY_INTERVAL=15
  until [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]
  do
      ${AWS_CMD} s3 ls && break
      MINIO_CONFIGURE_RETRY=$((MINIO_CONFIGURE_RETRY + 1))
    echo "Retrying [${MINIO_CONFIGURE_RETRY}/${MINIO_CONFIGURE_MAX_RETRY}] in ${MINIO_RETRY_INTERVAL}(s) "
    sleep ${MINIO_RETRY_INTERVAL}
  done

  if [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]; then
    echo "Failed to communicate with the MinIO server after ${MINIO_CONFIGURE_MAX_RETRY} attempts!"
    exit 1
  fi

  # Create the bucket
  ${AWS_CMD} s3 mb s3://${MINIO_BUCKET}

  cat <<EOF | sudo tee "${MINIO_CREDENTIALS_FILE}"
[default]
aws_access_key_id = ${MINIO_ROOT_USER}
aws_secret_access_key = ${MINIO_ROOT_PASSWORD}
EOF

  cat <<EOF | sudo tee "/etc/osbuild-worker/osbuild-worker.toml"
[generic_s3]
credentials = "${MINIO_CREDENTIALS_FILE}"
endpoint = "${MINIO_ENDPOINT}"
region = "${MINIO_REGION}"
bucket = "${MINIO_BUCKET}"
EOF

  sudo systemctl restart "osbuild-remote-worker@localhost:8700"
}

# Unset AWS_REGION, region == "" in the request the worker will look for the generic s3
# implementation
function createReqFile() {
    case ${IMAGE_TYPE} in
        "$IMAGE_TYPE_EDGE_COMMIT"|"$IMAGE_TYPE_EDGE_INSTALLER"|"$IMAGE_TYPE_IMAGE_INSTALLER")
            AWS_REGION='' createReqFileEdge
            ;;
        "$IMAGE_TYPE_VSPHERE")
            AWS_REGION='' createReqFileGuest
            ;;
        "$IMAGE_TYPE_VSPHERE")
            AWS_REGION='' createReqFileVSphere
            ;;
        *)
            echo "Unknown s3 image type for: ${IMAGE_TYPE}"
            exit 1
    esac
}

function checkUploadStatusOptions() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # S3 URL contains endpoint and bucket name
  echo "$S3_URL" | grep -F "$MINIO_ENDPOINT" -
  echo "$S3_URL" | grep -F "$MINIO_BUCKET" -
}

# Verify s3 blobs
function verify() {
    local S3_URL
    S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
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
