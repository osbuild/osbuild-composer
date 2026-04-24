#!/usr/bin/bash

#
# End-to-end functional test for bootc composes in a service deployment.
# Tests the full pipeline: Cloud API + JWT + private registry + AWS EC2
# executor + cloud upload target + image verification.
#
# Usage: api-bootc-service.sh <image-type>
#   e.g.: api-bootc-service.sh guest-image
#

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh
source /usr/libexec/tests/osbuild-composer/api/common/image-types.sh
source /usr/libexec/tests/osbuild-composer/api/common/bootc.sh
source /usr/libexec/tests/osbuild-composer/api/common/executor.sh

if (( $# != 1 )); then
    redprint "Usage: $0 <image-type>"
    redprint "Supported image types: ${IMAGE_TYPE_GUEST}, ${IMAGE_TYPE_BOOTABLE_CONTAINER_ISO}"
    exit 1
fi

IMAGE_TYPE="$1"

# Select handler based on image type
case "${IMAGE_TYPE}" in
    "$IMAGE_TYPE_GUEST")
        source /usr/libexec/tests/osbuild-composer/api/bootc/guest.s3.sh
        ;;
    "$IMAGE_TYPE_BOOTABLE_CONTAINER_ISO")
        source /usr/libexec/tests/osbuild-composer/api/bootc/container-iso.s3.sh
        ;;
    *)
        redprint "Unknown image type: ${IMAGE_TYPE}"
        exit 1
        ;;
esac

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Container image used for cloud provider CLI tools (used by sourced handlers)
export CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

# This test relies on podman-specific behavior (authfile + image exists checks).
if ! type -p podman 2>/dev/null >&2; then
    redprint "podman is required for api-bootc-service.sh."
    exit 2
fi
CONTAINER_RUNTIME=podman

#
# Resolve bootc container ref from Schutzfile
#
ARCH=$(uname -m)
BOOTC_CONTAINER_REF="${BOOTC_CONTAINER_REF_OVERRIDE:-$(jq -r \
    ".[\"${ID}-${VERSION_ID}\"].dependencies.bootc[\"${IMAGE_TYPE}\"][\"${ARCH}\"].base" \
    Schutzfile)}"

if [ -z "$BOOTC_CONTAINER_REF" ] || [ "$BOOTC_CONTAINER_REF" = "null" ]; then
    redprint "No bootc container ref found in Schutzfile for ${ID}-${VERSION_ID} / ${IMAGE_TYPE} / ${ARCH}"
    exit 1
fi
greenprint "Using bootc container ref: ${BOOTC_CONTAINER_REF}"

# For bootable-container-iso, optionally resolve the payload container ref
BOOTC_PAYLOAD_REF=""
if [ "${IMAGE_TYPE}" = "${IMAGE_TYPE_BOOTABLE_CONTAINER_ISO}" ]; then
    BOOTC_PAYLOAD_REF="${BOOTC_PAYLOAD_REF_OVERRIDE:-$(jq -r ".[\"${ID}-${VERSION_ID}\"].dependencies.bootc[\"${IMAGE_TYPE}\"][\"${ARCH}\"].payload" Schutzfile)}"

    if [ "$BOOTC_PAYLOAD_REF" = "null" ] || [ -z "$BOOTC_PAYLOAD_REF" ]; then
        BOOTC_PAYLOAD_REF=""
        greenprint "No payload ref configured, bootable-container-iso will be built without embedded payload"
    else
        greenprint "Using payload ref: ${BOOTC_PAYLOAD_REF}"
    fi
fi

# Extract registry host from the container ref (everything before the first /)
REGISTRY_HOST="${BOOTC_CONTAINER_REF%%/*}"

if [ -n "$BOOTC_PAYLOAD_REF" ]; then
    if [ "${REGISTRY_HOST}" != "${BOOTC_PAYLOAD_REF%%/*}" ]; then
        redprint "The base and payload container refs do not have the same registry host"
        exit 1
    fi
fi

#
# Verify environment
#
greenprint "Verifying environment"
checkEnv

#
# Provision the software under test (JWT mode).
# This sets up certificates, JWT composer/worker configs, mock auth servers,
# and starts Cloud API + remote worker units.
#
section_start "provision" "Provisioning with JWT authentication" true
/usr/libexec/osbuild-composer-test/provision.sh jwt
section_end "provision"

#
# Set up the database queue
#
section_start "provision-db" "Provisioning the database" true
DB_CONTAINER_NAME="osbuild-composer-db"
sudo "${CONTAINER_RUNTIME}" run -d --name "${DB_CONTAINER_NAME}" \
    --health-cmd "pg_isready -U postgres -d osbuildcomposer" --health-interval 2s \
    --health-timeout 2s --health-retries 10 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=foobar \
    -e POSTGRES_DB=osbuildcomposer \
    -p 5432:5432 \
    --net host \
    quay.io/osbuild/postgres:13-alpine

sudo "${CONTAINER_RUNTIME}" logs "${DB_CONTAINER_NAME}"

pushd "$(mktemp -d)"
sudo dnf install -y go
go mod init temp
go install github.com/jackc/tern@latest
PGUSER=postgres PGPASSWORD=foobar PGDATABASE=osbuildcomposer PGHOST=localhost PGPORT=5432 \
    "$(go env GOPATH)"/bin/tern migrate -m /usr/share/tests/osbuild-composer/schemas
popd
section_end "provision-db"

#
# Cleanup handler
#
WORKDIR=$(mktemp -d)
KILL_PIDS=()

function dump_db() {
    if ! sudo "${CONTAINER_RUNTIME}" inspect --format '{{.State.Running}}' "${DB_CONTAINER_NAME}" 2>/dev/null | grep -q true; then
        yellowprint "DB container ${DB_CONTAINER_NAME} is not running, skipping dump"
        return
    fi
    sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" \
        psql -U postgres -d osbuildcomposer -c "SELECT type, args, result FROM jobs" \
        | sudo tee "${ARTIFACTS}/build-result.txt" > /dev/null
}

function cleanups() {
    section_start "cleanup" "Cleaning up" true
    set +eu

    greenprint "Cleanup: killing background processes"
    for P in "${KILL_PIDS[@]}"; do
        sudo pkill -P "$P" 2>/dev/null || true
    done

    greenprint "Cleanup: handler cleanup (S3)"
    cleanup

    greenprint "Cleanup: dumping DB"
    dump_db

    greenprint "Cleanup: stopping DB container"
    sudo "${CONTAINER_RUNTIME}" kill "${DB_CONTAINER_NAME}"
    sudo "${CONTAINER_RUNTIME}" rm "${DB_CONTAINER_NAME}"

    greenprint "Cleanup: removing executor keypair"
    cleanupExecutor

    greenprint "Cleanup: removing workdir"
    sudo rm -rf "$WORKDIR"

    greenprint "Cleanup: stopping mock auth servers"
    /usr/libexec/osbuild-composer-test/run-mock-auth-servers.sh stop

    greenprint "Cleanup: done"
    set -eu
    section_end "cleanup"
}
trap cleanups EXIT

#
# Configure composer with JWT + DB + bootc remote container sources.
# Overwrites the config written by provision.sh to produce a single valid TOML
# file (duplicate [worker] sections would make the parser reject the file).
#
section_start "configure-composer" "Configuring osbuild-composer" true
sudo tee /etc/osbuild-composer/osbuild-composer.toml > /dev/null <<EOF
ignore_missing_repos = true

# Despite the name, [koji] configures the Cloud API (osbuild-composer-api.socket, port 443).
# Without it, defaults (enable_tls=true, enable_mtls=true) would require mTLS on the API.
[koji]
enable_tls = false
enable_mtls = false
enable_jwt = true
jwt_keys_urls = ["https://localhost:8082/certs"]
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
jwt_acl_file = ""
jwt_tenant_provider_fields = ["rh-org-id"]

# [worker] configures the remote worker API (osbuild-remote-worker.socket, port 8700).
[worker]
enable_artifacts = false
enable_tls = true
enable_mtls = false
enable_jwt = true
jwt_keys_urls = ["https://localhost:8082/certs"]
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
jwt_tenant_provider_fields = ["rh-org-id"]
pg_host = "localhost"
pg_port = "5432"
pg_database = "osbuildcomposer"
pg_user = "postgres"
pg_password = "foobar"
pg_ssl_mode = "disable"
pg_max_conns = 10

[bootc]
use_remote_container_source = true
EOF

sudo systemctl restart osbuild-composer
section_end "configure-composer"

#
# Configure container registry authentication
#
section_start "configure-registry-auth" "Configuring container registry authentication" true
CONTAINER_AUTH_FILE="/etc/osbuild-worker/containerauth.json"
sudo "${CONTAINER_RUNTIME}" login \
    --authfile "${CONTAINER_AUTH_FILE}" \
    --username "${BOOTC_FOUNDRY_DERIVED_CONTAINERS_REGISTRY_USER}" \
    --password "${BOOTC_FOUNDRY_DERIVED_CONTAINERS_REGISTRY_PASS}" \
    "${REGISTRY_HOST}"

# Set REGISTRY_AUTH_FILE for all worker instances via systemd drop-in
WORKER_DROPIN_DIR="/etc/systemd/system/osbuild-remote-worker@.service.d"
sudo mkdir -p "${WORKER_DROPIN_DIR}"
sudo tee "${WORKER_DROPIN_DIR}/registry-auth.conf" > /dev/null <<EOF
[Service]
Environment="REGISTRY_AUTH_FILE=${CONTAINER_AUTH_FILE}"
EOF

section_end "configure-registry-auth"

#
# Install cloud provider client tools (from handler)
#
section_start "install-client" "Installing cloud provider client tools" true
installClient
section_end "install-client"

#
# Configure worker: executor (JWT auth and AWS credentials already configured by provision.sh)
#
section_start "configure-worker" "Configuring osbuild-worker" true

# Set up executor keypair. Key name is exported as EXECUTOR_KEY_NAME.
setupExecutorKeypair

# Append executor configuration to the worker config
sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null <<EOF

[osbuild_executor]
type = "aws.ec2"
key_name = "${EXECUTOR_KEY_NAME}"

[bootc_info_resolve]
cleanup_images = true
EOF

sudo systemctl daemon-reload
sudo systemctl restart "osbuild-remote-worker@*"

section_end "configure-worker"

# Tail worker journal for diagnostics
sudo journalctl -af -n 1 -u "osbuild-remote-worker@*" &
KILL_PIDS+=("$!")

#
# Verify openapi endpoint is accessible
#
section_start "verify-openapi" "Verifying Cloud API is accessible" true
TOKEN=$(curl --request POST \
    --data "grant_type=refresh_token" \
    --data "refresh_token=$(cat /etc/osbuild-worker/token)" \
    --header "Content-Type: application/x-www-form-urlencoded" \
    --silent \
    --show-error \
    --fail \
    localhost:8081/token | jq -r .access_token)

curl \
    --silent \
    --show-error \
    --fail \
    --header "Authorization: Bearer ${TOKEN}" \
    http://localhost:443/api/image-builder-composer/v2/openapi | jq .
section_end "verify-openapi"

#
# Pre-compose verification: container ref should NOT be in local storage
#
section_start "verify-container-ref-pre-compose" "Verifying container ref is NOT in local storage" true
if sudo podman image exists "${BOOTC_CONTAINER_REF}" 2>/dev/null; then
    redprint "Container ref ${BOOTC_CONTAINER_REF} already exists in local storage!"
    exit 1
fi
section_end "verify-container-ref-pre-compose"

#
# Compose execution
#
REQUEST_FILE="${WORKDIR}/compose_request.json"
export REQUEST_FILE WORKDIR ARCH IMAGE_TYPE BOOTC_CONTAINER_REF BOOTC_PAYLOAD_REF

greenprint "Creating compose request"
createReqFile

function sendCompose() {
    local OUTPUT HTTPSTATUS
    OUTPUT=$(mktemp)
    HTTPSTATUS=$(curl \
        --silent \
        --show-error \
        --header "Authorization: Bearer ${TOKEN}" \
        --header 'Content-Type: application/json' \
        --request POST \
        --data @"$1" \
        --write-out '%{http_code}' \
        --output "$OUTPUT" \
        http://localhost:443/api/image-builder-composer/v2/compose)

    if [ "$HTTPSTATUS" != "201" ]; then
        redprint "Sending compose request failed:"
        cat "$OUTPUT"
    fi

    test "$HTTPSTATUS" = "201"

    COMPOSE_ID=$(jq -r '.id' "$OUTPUT")
}

function waitForState() {
    local DESIRED_STATE="${1:-success}"
    local MAX_ITERATIONS=120
    local ITERATIONS=0
    local OUTPUT COMPOSE_STATUS

    local SECTION_ID
    SECTION_ID="wait-for-state-$(uuidgen)"
    section_start "${SECTION_ID}" "Waiting for compose to reach state '${DESIRED_STATE}'" true
    while [ "$ITERATIONS" -lt "$MAX_ITERATIONS" ]
    do
        ITERATIONS=$((ITERATIONS + 1))
        OUTPUT=$(curl \
            --silent \
            --show-error \
            --fail \
            --header "Authorization: Bearer ${TOKEN}" \
            http://localhost:443/api/image-builder-composer/v2/composes/"$COMPOSE_ID")

        COMPOSE_STATUS=$(echo "$OUTPUT" | jq -r '.image_status.status')
        UPLOAD_STATUS=$(echo "$OUTPUT" | jq -r '.image_status.upload_status.status')
        UPLOAD_OPTIONS=$(echo "$OUTPUT" | jq -r '.image_status.upload_status.options')

        case "$COMPOSE_STATUS" in
            "$DESIRED_STATE"|"success")
                break
                ;;
            "pending"|"building"|"uploading"|"registering")
                ;;
            "failure")
                redprint "Image compose failed"
                echo "API output: $OUTPUT"
                dump_db
                exit 1
                ;;
            *)
                redprint "API returned unexpected image_status.status value: '$COMPOSE_STATUS'"
                echo "API output: $OUTPUT"
                dump_db
                exit 1
                ;;
        esac

        sleep 30
    done

    if [ "$ITERATIONS" -ge "$MAX_ITERATIONS" ]; then
        redprint "Timed out waiting for compose to reach state '${DESIRED_STATE}' after $((MAX_ITERATIONS * 30)) seconds"
        dump_db
        exit 1
    fi

    export UPLOAD_STATUS UPLOAD_OPTIONS
    section_end "${SECTION_ID}"
}

function verifyManifestContainerSourceType() {
    MANIFESTS=$(curl \
        --silent \
        --show-error \
        --fail \
        --header "Authorization: Bearer ${TOKEN}" \
        "http://localhost:443/api/image-builder-composer/v2/composes/$COMPOSE_ID/manifests")
    verifyContainerSourceType "$MANIFESTS" "remote"
}

greenprint "Sending bootc compose"
sendCompose "$REQUEST_FILE"

#
# Wait for the worker to pick up the compose job before looking for the
# executor instance it creates.
#
waitForState "building"

#
# The worker creates the executor EC2 instance when it picks up the osbuild
# job from the compose above. Wait for it to appear, then provision and start it.
#
section_start "setup-executor" "Setting up executor" true
waitForExecutorInstance
verifyExecutorNetworkIsolation
provisionExecutor
startExecutor
section_end "setup-executor"

waitForState "success"

test "$UPLOAD_STATUS" = "success"

#
# Post-compose verification: container ref should NOT be in local storage (cleanup enabled)
#
section_start "verify-container-ref-post-compose" "Verifying container ref is NOT in local storage" true
if sudo podman image exists "${BOOTC_CONTAINER_REF}" 2>/dev/null; then
    redprint "Container ref ${BOOTC_CONTAINER_REF} was NOT cleaned up from local storage by BootcInfoResolveJob!"
    exit 1
fi
section_end "verify-container-ref-post-compose"

#
# Verify upload status options (from handler)
#
greenprint "Checking upload status options"
checkUploadStatusOptions

#
# Verify the container source type in the compose manifest is correct
#
section_start "verify-manifest" "Verifying container source type in compose manifest" true
verifyManifestContainerSourceType
section_end "verify-manifest"

#
# Verify the built image (from handler)
#
section_start "verify-image" "Verifying built image" true
verify
section_end "verify-image"

greenprint "✅ DONE"
exit 0
