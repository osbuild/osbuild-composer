#!/bin/bash
set -euo pipefail

# Colorful output.
function test_utils::greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

function test_utils::set_container_runtime {
# Check available container runtime
    if which podman 2>/dev/null >&2; then
        export CONTAINER_RUNTIME="podman"
    elif which docker 2>/dev/null >&2; then
        export CONTAINER_RUNTIME="docker"
    else
        echo No container runtime found, install podman or docker.
        exit 2
    fi
}

function test_utils::generate_test_id {
    # Generate a string, which can be used as a predictable resource name,
    # especially when running the test in CI where we may need to clean up
    # resources in case the test unexpectedly fails or is canceled
    CI="${CI:-false}"
    if [[ "$CI" == true ]]; then
      # in CI, imitate GenerateCIArtifactName() from internal/test/helpers.go
      echo "$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_BUILD_ID"
    else
      # if not running in Jenkins, generate ID not relying on specific env variables
      uuidgen;
    fi
}

# Check for the smoke test file on the instance that we start.
function test_utils::smoke_test_check {
    local ssh_key="$1"
    local public_ip="$2"
    # Ensure the ssh key has restricted permissions.
    SMOKE_TEST=$(sudo ssh -i "${ssh_key}" redhat@"${public_ip}" 'cat /etc/smoke-test.txt')
    if [[ $SMOKE_TEST == smoke-test ]]; then
        echo 1
    else
        echo 0
    fi
}

# Get the compose log.
function test_utils::get_compose_log {
    local compose_id=$1
    local compose_type=$2
    LOG_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-${compose_type}.log

    # Download the logs.
    sudo composer-cli compose log "$compose_id" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
function test_utils::get_compose_metadata {
    local compose_id=$1
    local compose_type=$2
    METADATA_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-${compose_type}.json

    # Download the metadata.
    sudo composer-cli compose metadata "$compose_id" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${compose_id}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

function test_utils::is_weldr_client_installed {
    if rpm --quiet -q weldr-client; then
        echo true
    else
        echo false
    fi
}

function test_utils::get_build_info {
    local key="$1"
    local fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
    fi
    jq -r "${key}" "${fname}"
}

function test_utils::install_oci_client {
    local containerfile
    containerfile=$(cat <<-EOF
		FROM registry.access.redhat.com/ubi8/python-38
		RUN bash -c "\$(curl -L https://raw.githubusercontent.com/oracle/oci-cli/master/scripts/install/install.sh)" -- --accept-all-defaults
		ENTRYPOINT ["oci"]
		EOF
    )
    test_utils::set_container_runtime
    if ! hash oci; then
        echo "Using 'oci' cli from a container"
        image_id=$("${CONTAINER_RUNTIME}" build --quiet -f <(echo "${containerfile}"))
        OCI_CMD="${CONTAINER_RUNTIME} run \
            --user $UID --userns=keep-id \
            -e OCI_CLI_SUPPRESS_FILE_PERMISSIONS_WARNING=True \
            -e OCI_CLI_USER=${OCI_CLI_USER} \
            -e OCI_CLI_REGION=${OCI_CLI_REGION} \
            -e OCI_CLI_TENANCY=${OCI_CLI_TENANCY} \
            -e OCI_CLI_FINGERPRINT=${OCI_CLI_FINGERPRINT} \
            -e OCI_BUCKET=${OCI_BUCKET} \
            -e OCI_NAMESPACE=${OCI_NAMESPACE} \
            -e OCI_COMPARTMENT=${OCI_COMPARTMENT} \
            -e OCI_CLI_KEY_FILE=/.oci_key \
            -v ${OCI_CLI_KEY_FILE}:/.oci_key:Z \
            -v ${TEMPDIR}:${TEMPDIR}:Z \
            -v ${SSH_DATA_DIR}:${SSH_DATA_DIR}:Z \
            ${image_id} --output json"
    else
        echo "Using pre-installed 'oci' from the system"
        OCI_CMD="oci --region $OCI_CLI_REGION --output json"
    fi
    export OCI_CMD
    $OCI_CMD --version
}
