#!/bin/bash
set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Container images
CONTAINER_MINIO_SERVER="quay.io/minio/minio:latest"
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

MINIO_CONTAINER_NAME="minio-server"
MINIO_ROOT_USER="X29DU5Q6C5NKDQ8PLGVT"
MINIO_ROOT_PASSWORD=$(date +%s | sha256sum | base64 | head -c 32 ; echo)
MINIO_BUCKET="ci-test"
MINIO_REGION="us-east-1"

CERTGEN_VERSION="v1.2.0"

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
mkdir -p "${ARTIFACTS}"

# ============================================================
# Shared setup
# ============================================================

/usr/libexec/osbuild-composer-test/provision.sh none

# Check available container runtime
if type -p podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME=docker
else
    echo "No container runtime found, install podman or docker."
    exit 2
fi

TEST_ID=$(uuidgen)
CI="${CI:-false}"
if [[ "${CI}" == true ]]; then
    TEST_ID="${DISTRO_CODE}-${ARCH}-${CI_COMMIT_BRANCH}-${CI_JOB_ID}"
fi

TEMPDIR=$(mktemp -d)
CERTS_DIR=$(sudo mktemp -d -p /var/lib s3-certs.XXXXXX)
sudo chmod 755 "${CERTS_DIR}"

function global_cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    stop_minio || true
    sudo rm -rf "${TEMPDIR}"
    sudo rm -rf "${CERTS_DIR}"
}
trap global_cleanup EXIT

sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_MINIO_SERVER}"
sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_IMAGE_CLOUD_TOOLS}"

# Generate TLS certs for HTTPS variants (reused across both HTTPS tests)
# Certs must be outside /tmp because osbuild-worker has PrivateTmp=true
pushd "${TEMPDIR}"
curl -L -o certgen "https://github.com/minio/certgen/releases/download/${CERTGEN_VERSION}/certgen-linux-amd64"
chmod +x certgen
./certgen -host localhost
sudo mv private.key public.crt "${CERTS_DIR}"
sudo chmod 644 "${CERTS_DIR}"/*
popd

# Push the blueprint once -- osbuild caching means subsequent builds are fast
BLUEPRINT_FILE="${TEMPDIR}/blueprint.toml"
BLUEPRINT_NAME="empty"
tee "${BLUEPRINT_FILE}" > /dev/null << EOF
name = "${BLUEPRINT_NAME}"
description = "A base system with bash"
version = "0.0.1"
EOF

greenprint "Preparing blueprint"
sudo composer-cli blueprints push "${BLUEPRINT_FILE}"
sudo composer-cli blueprints depsolve "${BLUEPRINT_NAME}"

# ============================================================
# Helper functions
# ============================================================

start_minio() {
    local mode="${1}"  # "http" or "https"

    greenprint "Starting MinIO server (${mode})"

    local run_args=(
        --rm -d
        --name "${MINIO_CONTAINER_NAME}"
        -p 9000:9000
        -e MINIO_BROWSER=off
        -e "MINIO_ROOT_USER=${MINIO_ROOT_USER}"
        -e "MINIO_ROOT_PASSWORD=${MINIO_ROOT_PASSWORD}"
    )

    if [[ "${mode}" == "https" ]]; then
        run_args+=(-v "${CERTS_DIR}:/root/.minio/certs:z")
    fi

    sudo "${CONTAINER_RUNTIME}" run "${run_args[@]}" "${CONTAINER_MINIO_SERVER}" server /data

    # Wait for MinIO to become ready
    local retry=0
    local max_retry=5
    local interval=15
    local s3_cmd
    if [[ "${mode}" == "https" ]]; then
        s3_cmd=$(build_aws_s3_cmd "${mode}" "${CERTS_DIR}/public.crt")
    else
        s3_cmd=$(build_aws_s3_cmd "${mode}" "")
    fi

    until [ "${retry}" -ge "${max_retry}" ]; do
        ${s3_cmd} ls && break
        retry=$((retry + 1))
        echo "Retrying [${retry}/${max_retry}] in ${interval}(s)"
        sleep "${interval}"
    done

    if [ "${retry}" -ge "${max_retry}" ]; then
        echo "Failed to communicate with MinIO after ${max_retry} attempts!"
        exit 1
    fi

    # Create the bucket
    ${s3_cmd} mb "s3://${MINIO_BUCKET}"
}

stop_minio() {
    greenprint "Stopping MinIO server"
    sudo "${CONTAINER_RUNTIME}" kill "${MINIO_CONTAINER_NAME}" 2>/dev/null || true
    sudo "${CONTAINER_RUNTIME}" rm -f "${MINIO_CONTAINER_NAME}" 2>/dev/null || true
}

build_aws_s3_cmd() {
    local mode="${1}"       # "http" or "https"
    local ca_bundle="${2}"  # path to CA bundle, "skip", or ""

    local endpoint="${mode}://localhost:9000"
    local cmd="sudo ${CONTAINER_RUNTIME} run --rm --network=host"
    cmd="${cmd} -e AWS_ACCESS_KEY_ID=${MINIO_ROOT_USER}"
    cmd="${cmd} -e AWS_SECRET_ACCESS_KEY=${MINIO_ROOT_PASSWORD}"

    if [[ -n "${ca_bundle}" && "${ca_bundle}" != "skip" ]]; then
        cmd="${cmd} -v ${ca_bundle}:${ca_bundle}:z"
    fi

    cmd="${cmd} ${CONTAINER_IMAGE_CLOUD_TOOLS}"
    cmd="${cmd} aws --region ${MINIO_REGION} --endpoint-url ${endpoint}"

    if [[ -n "${ca_bundle}" ]]; then
        if [[ "${ca_bundle}" == "skip" ]]; then
            cmd="${cmd} --no-verify-ssl"
        else
            cmd="${cmd} --ca-bundle ${ca_bundle}"
        fi
    fi

    cmd="${cmd} s3"
    echo "${cmd}"
}

write_provider_config() {
    local config_file="${1}"
    local mode="${2}"       # "http" or "https"
    local test_key="${3}"
    local ca_bundle="${4}"  # path to CA bundle, "skip", or ""

    local endpoint="${mode}://localhost:9000"

    tee "${config_file}" > /dev/null << EOF
provider = "generic.s3"

[settings]
endpoint = "${endpoint}"
accessKeyID = "${MINIO_ROOT_USER}"
secretAccessKey = "${MINIO_ROOT_PASSWORD}"
bucket = "${MINIO_BUCKET}"
region = "${MINIO_REGION}"
key = "${test_key}"
EOF

    if [[ -n "${ca_bundle}" ]]; then
        if [[ "${ca_bundle}" == "skip" ]]; then
            echo "skip_ssl_verification = true" >> "${config_file}"
        else
            echo "ca_bundle = \"${ca_bundle}\"" >> "${config_file}"
        fi
    fi
}

trigger_compose() {
    local test_key="${1}"
    local config_file="${2}"

    local compose_start="${TEMPDIR}/compose-start-${test_key}.json"

    greenprint "Starting compose for ${test_key}"
    sudo composer-cli --json compose start "${BLUEPRINT_NAME}" qcow2 "${test_key}" "${config_file}" | tee "${compose_start}"

    local compose_id
    compose_id=$(get_compose_id "${compose_start}")

    local compose_status
    compose_status=$(wait_for_compose "${compose_id}")

    # Save compose log
    sudo composer-cli compose log "${compose_id}" | tee "${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${test_key}.log" > /dev/null

    # "success" is the new status string from weldr-client v36.0+ using the cloud API since Fedora 44.
    if [[ "${compose_status}" != FINISHED ]] && [[ "${compose_status}" != "success" ]]; then
        redprint "Compose ${compose_id} failed with status: ${compose_status}"
        exit 1
    fi

    greenprint "Compose ${compose_id} finished successfully"
    sudo composer-cli compose delete "${compose_id}" > /dev/null
}

verify_image() {
    local s3_cmd="${1}"
    local object_key="${2}"
    local curl_opts="${3:-}"

    greenprint "Verifying uploaded image: ${object_key}"

    # Confirm the object exists in the bucket
    if ! ${s3_cmd} ls "s3://${object_key}"; then
        redprint "Image not found in S3: ${object_key}"
        exit 1
    fi

    # Download via presigned URL
    local presign_url
    presign_url=$(${s3_cmd} presign "s3://${object_key}")

    local image_path="${TEMPDIR}/downloaded-image.qcow2"
    # shellcheck disable=SC2086
    curl ${curl_opts} -o "${image_path}" "${presign_url}"

    # Validate that the downloaded file is a proper qcow2 image
    qemu-img info "${image_path}" | grep -q "file format: qcow2"
    qemu-img check "${image_path}"
    greenprint "Image verification passed"

    # Cleanup downloaded image and S3 object
    rm -f "${image_path}"
    ${s3_cmd} rm "s3://${object_key}"
}

# ============================================================
# Test cases
# ============================================================

test_s3_http() {
    local test_key="${TEST_ID}-http"
    local config_file="${TEMPDIR}/provider-http.toml"
    local s3_cmd
    local object_key="${MINIO_BUCKET}/${test_key}-disk.qcow2"

    section_start "test_s3_http" "Test: generic S3 over HTTP" false

    start_minio "http"
    s3_cmd=$(build_aws_s3_cmd "http" "")
    write_provider_config "${config_file}" "http" "${test_key}" ""
    trigger_compose "${test_key}" "${config_file}"
    verify_image "${s3_cmd}" "${object_key}" ""
    stop_minio

    section_end "test_s3_http"
}

test_s3_https_secure() {
    local test_key="${TEST_ID}-https-secure"
    local config_file="${TEMPDIR}/provider-https-secure.toml"
    local ca_bundle="${CERTS_DIR}/public.crt"
    local s3_cmd
    local object_key="${MINIO_BUCKET}/${test_key}-disk.qcow2"

    section_start "test_s3_https_secure" "Test: generic S3 over HTTPS (CA verified)" false

    start_minio "https"
    s3_cmd=$(build_aws_s3_cmd "https" "${ca_bundle}")
    write_provider_config "${config_file}" "https" "${test_key}" "${ca_bundle}"
    trigger_compose "${test_key}" "${config_file}"
    verify_image "${s3_cmd}" "${object_key}" "--cacert ${ca_bundle}"
    stop_minio

    section_end "test_s3_https_secure"
}

test_s3_https_insecure() {
    local test_key="${TEST_ID}-https-insecure"
    local config_file="${TEMPDIR}/provider-https-insecure.toml"
    local ca_bundle="${CERTS_DIR}/public.crt"
    local s3_cmd
    local object_key="${MINIO_BUCKET}/${test_key}-disk.qcow2"

    section_start "test_s3_https_insecure" "Test: generic S3 over HTTPS (skip verification)" false

    start_minio "https"
    s3_cmd=$(build_aws_s3_cmd "https" "${ca_bundle}")
    write_provider_config "${config_file}" "https" "${test_key}" "skip"
    trigger_compose "${test_key}" "${config_file}"
    verify_image "${s3_cmd}" "${object_key}" "--cacert ${ca_bundle}"
    stop_minio

    section_end "test_s3_https_insecure"
}

# ============================================================
# Main
# ============================================================

greenprint "Running generic S3 integration tests"

test_s3_http
test_s3_https_secure
test_s3_https_insecure

greenprint "All generic S3 tests passed"

