#!/usr/bin/bash

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh
source /usr/libexec/tests/osbuild-composer/api/common/bootc.sh
source /usr/libexec/tests/osbuild-composer/api/common/composer-db.sh
source /usr/libexec/tests/osbuild-composer/api/common/common.sh

/usr/libexec/osbuild-composer-test/provision.sh

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

function dump_db() {
  # Save the result, including the manifest, for the job, straight from the db
  sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c "SELECT type, args, result FROM jobs" \
    | sudo tee "${ARTIFACTS}/build-result.txt" > /dev/null
}

KILL_PIDS=()
function cleanups() {
  greenprint "Cleaning up"
  set +eu

  # save manifest
  dump_db

  teardown_db

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
  set -eu
}
trap cleanups EXIT

# Start the db
setup_db

BOOTC_USE_REMOTE_CONTAINER_SOURCE="${BOOTC_USE_REMOTE_CONTAINER_SOURCE:-false}"

write_tls_composer_config
cat <<EOF | sudo tee -a "/etc/osbuild-composer/osbuild-composer.toml"
[bootc]
use_remote_container_source = ${BOOTC_USE_REMOTE_CONTAINER_SOURCE}
EOF

sudo systemctl restart osbuild-composer

curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/image-builder-composer/v2/openapi | jq .


# verify the container source type in the compose manifest is correct
function verifyManifestContainerSourceType() {
    MANIFESTS=$(curl \
        --silent \
        --show-error \
        --cacert /etc/osbuild-composer/ca-crt.pem \
        --key /etc/osbuild-composer/client-key.pem \
        --cert /etc/osbuild-composer/client-crt.pem \
        "https://localhost/api/image-builder-composer/v2/composes/$COMPOSE_ID/manifests")
    echo "compose MANIFESTS:"
    echo "$MANIFESTS"
    if [ "$BOOTC_USE_REMOTE_CONTAINER_SOURCE" = "true" ]; then
        verifyContainerSourceType "$MANIFESTS" "remote"
    else
        verifyContainerSourceType "$MANIFESTS" "local"
    fi
}

function verifyImageDownload() {
    ARTIFACT_PATH=$(echo "$UPLOAD_OPTIONS" | jq -r '.artifact_path')
    test -n "$ARTIFACT_PATH"
    test "$ARTIFACT_PATH" != "null"

    DOWNLOAD_OUTPUT="${WORKDIR}/downloaded-image"
    HTTPSTATUS=$(curl \
        --silent \
        --show-error \
        --cacert /etc/osbuild-composer/ca-crt.pem \
        --key /etc/osbuild-composer/client-key.pem \
        --cert /etc/osbuild-composer/client-crt.pem \
        --write-out '%{http_code}' \
        --output "$DOWNLOAD_OUTPUT" \
        "https://localhost/api/image-builder-composer/v2/composes/$COMPOSE_ID/download")
    echo "Download HTTP status: $HTTPSTATUS"
    test "$HTTPSTATUS" = "200"
    test -s "$DOWNLOAD_OUTPUT"

    # sudo is needed to access the artifact in the worker's private directory
    ARTIFACT_SIZE=$(sudo stat --format='%s' "$ARTIFACT_PATH")
    DOWNLOAD_SIZE=$(stat --format='%s' "$DOWNLOAD_OUTPUT")
    echo "Artifact size: $ARTIFACT_SIZE"
    echo "Download size: $DOWNLOAD_SIZE"
    test "$ARTIFACT_SIZE" = "$DOWNLOAD_SIZE"
}

WORKDIR=$(mktemp -d)
REQ="${WORKDIR}/compose_request.json"
ARCH=$(uname -m)

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
KILL_PIDS+=("$!")

BOOTC_IMAGE_TYPES=("guest-image" "aws")

for IMAGE_TYPE in "${BOOTC_IMAGE_TYPES[@]}"; do
    greenprint "Testing bootc compose with image_type=${IMAGE_TYPE}"

    cat > "$REQ" << EOF
{
  "bootc": {
    "reference": "quay.io/centos-bootc/centos-bootc:stream9"
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "$IMAGE_TYPE",
    "repositories": [],
    "upload_targets": [{"type": "local", "upload_options": {}}]
  }
}
EOF

    sendCompose "$REQ"
    waitForState
    echo "compose status:"
    curl \
        --show-error \
        --cacert /etc/osbuild-composer/ca-crt.pem \
        --key /etc/osbuild-composer/client-key.pem \
        --cert /etc/osbuild-composer/client-crt.pem \
        "https://localhost/api/image-builder-composer/v2/composes/$COMPOSE_ID"
    test "$UPLOAD_STATUS" = "success"

    verifyImageDownload
    verifyManifestContainerSourceType
done
