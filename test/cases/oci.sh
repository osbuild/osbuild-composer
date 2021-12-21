#!/bin/bash
set -euxo pipefail

source "$(dirname "$(realpath -s "$0")")"/test-utils.sh
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Jenkins sets WORKSPACE to the job workspace, but if this script runs
# outside of Jenkins, we can set up a temporary directory instead.
WORKSPACE=${WORKSPACE:-$(mktemp -d)}
# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh
TEST_ID=$(test_utils::generate_test_id)
TEMPDIR=$(mktemp -d)
OCI_CONFIG=${TEMPDIR}/oci.toml
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${TEST_ID}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${TEST_ID}.json
OCI_IMAGE_DATA=${TEMPDIR}/oci-image-data-${TEST_ID}.json
SSH_DATA_DIR=$(tools/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa


function cleanup() {
    set +e +o pipefail
    teardown
    # kill all child process by process group id
    kill -EXIT -- -"$(ps --no-headers -o pgid:1 -p $$)"
    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

function check_mandatory_inputs {
    MANDATORY_ENV_VARS=(
        OCI_CLI_USER \
        OCI_CLI_TENANCY \
        OCI_CLI_FINGERPRINT \
        OCI_CLI_REGION \
        OCI_BUCKET \
        OCI_NAMESPACE \
        OCI_COMPARTMENT \
        OCI_CLI_KEY_FILE \
    )
    local r=""
    for ev in "${MANDATORY_ENV_VARS[@]}";do
        [[ -v $ev ]] || { echo "$ev is a mandatory env variable. \
            Please export or set during invocation (e.g \$ OCI_CLI_KEY_FILE=/path/to/key $0)" && r="false"; }
    done
    [[ -z "$r" ]]
}

function setup_files() {
    # Write an OCI TOML file
    tee "$OCI_CONFIG" > /dev/null <<-EOF
		provider = "oci"

		[settings]
		user = "${OCI_CLI_USER/$'\n'/}"
		tenancy = "${OCI_CLI_TENANCY/$'\n'/}"
		fingerprint = "${OCI_CLI_FINGERPRINT/$'\n'/}"
		region = "${OCI_CLI_REGION/$'\n'/}"
		bucket = "${OCI_BUCKET/$'\n'/}"
		namespace = "${OCI_NAMESPACE/$'\n'/}"
		compartment = "${OCI_COMPARTMENT/$'\n'/}"
		private_key = '''
		$(cat "${OCI_CLI_KEY_FILE}")
		'''
		EOF

    # Write a basic blueprint for our image.
    tee "$BLUEPRINT_FILE" > /dev/null <<-EOF
		name = "bash"
		description = "A base system with bash"
		version = "0.0.1"

		[[packages]]
		name = "bash"

		[customizations.services]
		enabled = ["sshd", "cloud-init", "cloud-init-local", "cloud-config", "cloud-final"]
		EOF
}

function prepare_blueprint() {
    test_utils::greenprint "📋 Preparing blueprint"
    sudo composer-cli blueprints push "$BLUEPRINT_FILE"
    sudo composer-cli blueprints depsolve bash
}

function compose() {
    # Start the compose and upload to OCI.
    test_utils::greenprint "🚀 Starting compose"
    sudo composer-cli --json compose start bash oci "$TEST_ID" "$OCI_CONFIG" | tee "$COMPOSE_START"
    COMPOSE_ID=$(test_utils::get_build_info ".build_id" "$COMPOSE_START")

    # Wait for the compose to finish.
    test_utils::greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
        COMPOSE_STATUS=$(test_utils::get_build_info ".queue_status" "$COMPOSE_INFO")

        # Is the compose finished?
        if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 30
    done

    # Capture the compose logs from osbuild.
    test_utils::greenprint "💬 Getting compose log and metadata"
    test_utils::get_compose_log "$COMPOSE_ID" "oci"
    test_utils::get_compose_metadata "$COMPOSE_ID" "oci"

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi
}

function get_availability_domain_by_shape {
    for ad in $($OCI_CMD iam availability-domain list -c "$OCI_COMPARTMENT" | jq -r '.data[].name');do
        if [ "$($OCI_CMD compute shape list -c "$OCI_COMPARTMENT" --availability-domain "$ad" | jq --arg SHAPE "$1" -r '.data[]|select(.shape==$SHAPE)|.shape')" == "$1" ];then
            echo "$ad"
            return
        fi
    done
    return 1
}

function run_instance_from_image() {
    # Find the image that we made in OCI.
    test_utils::greenprint "🔍 Search for the created OCI image"
    RET=$($OCI_CMD search resource structured-search \
        --query-text "query image resources where (freeformTags.key = 'Name' && freeformTags.value = '${TEST_ID}' )")
    echo "$RET"
    echo "$RET" | jq '.data.items[0]' \
        | tee "$OCI_IMAGE_DATA" > /dev/null

    OCI_IMAGE_ID=$(jq -r '.identifier' "$OCI_IMAGE_DATA")
    SHAPE="VM.Standard.E2.1.Micro"
    OCI_AVAILABILITY_DOMAIN="$(get_availability_domain_by_shape "$SHAPE")"
    OCI_SUBNET=$($OCI_CMD network subnet list -c "$OCI_COMPARTMENT" | jq -r '.data[0]|.id')

    # Build instance in OCI with our image.
    test_utils::greenprint "👷🏻 Building instance in OCI"
    INSTANCE=$($OCI_CMD compute instance launch  \
        --shape $SHAPE \
        -c "${OCI_COMPARTMENT}" \
        --availability-domain "${OCI_AVAILABILITY_DOMAIN}" \
        --subnet-id "${OCI_SUBNET}" \
        --image-id "${OCI_IMAGE_ID}" \
        --freeform-tags "{\"Name\": \"${TEST_ID}\", \"gitlab-ci-test\": \"true\"}" \
        --user-data-file "${SSH_DATA_DIR}"/user-data)

    INSTANCE_ID="$(echo "$INSTANCE" | jq -r '.data.id')"

    # Wait for the instance to finish building.
    test_utils::greenprint "⏱ Waiting for OCI instance to be marked as running"
    while true; do
        INSTANCE=$($OCI_CMD compute instance get --instance-id "$INSTANCE_ID")
        if [[ $(echo "$INSTANCE" | jq -r '.data."lifecycle-state"') == RUNNING ]]; then
            break
        fi
        sleep 10
    done

    # Get data about the instance we built.
    PUBLIC_IP=$($OCI_CMD compute instance list-vnics \
        --instance-id "$INSTANCE_ID" |  jq -r '.data[0]."public-ip"')

    # Wait for the node to come online.
    test_utils::greenprint "⏱ Waiting for OCI instance to respond to ssh"
    for (( i=0 ; i<30; i++ )); do
        if ssh-keyscan "$PUBLIC_IP" > /dev/null 2>&1; then
            echo "SSH is up!"
            ssh-keyscan "$PUBLIC_IP" | sudo tee -a /root/.ssh/known_hosts
            break
        fi

        # ssh-keyscan has a 5 second timeout by default, so the pause per loop
        # is 10 seconds when you include the following `sleep`.
        echo "Retrying in 5 seconds..."
        sleep 5
    done
}

function smoke_tests() {
    # Check for our smoke test file.
    test_utils::greenprint "🛃 Checking for smoke test file"
    for (( i=0; i<10; i++ )); do
        RESULTS="$(test_utils::smoke_test_check "$SSH_KEY" "$PUBLIC_IP")"
        if [[ $RESULTS == 1 ]]; then
            echo "Smoke test passed! 🥳"
            break
        fi
        sleep 5
    done

    # Ensure the image was properly tagged.
    IMAGE_TAG=$($OCI_CMD compute image get --image-id "${OCI_IMAGE_ID}" | jq -r '.data."freeform-tags".Name')
    if [[ ! $IMAGE_TAG == "${TEST_ID}" ]]; then
        RESULTS=0
        echo "image doesn't have the right tag? image_tag = $IMAGE_TAG and test td ${TEST_ID}"
    fi
}

function watch_journal() {
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    [[ -n "${WORKER_UNIT}" ]] && journalctl -af -n 1 -u "${WORKER_UNIT}" &
}

function teardown() {
    test_utils::greenprint "🧼 Cleaning up"
    $OCI_CMD compute instance terminate --instance-id "${INSTANCE_ID}" --force
    $OCI_CMD compute image delete --image-id "${OCI_IMAGE_ID}" --force
    # Also delete the compose so we don't run out of disk space
    sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
}

check_mandatory_inputs
test_utils::install_oci_client
setup_files
prepare_blueprint
compose
run_instance_from_image
smoke_tests
watch_journal
teardown

# Use the return code of the smoke test to determine if we passed or failed.
# On rhel continue with the cloudapi test
if [[ $RESULTS == 1 ]]; then
    test_utils::greenprint "💚 Success"
    exit 0
else
    test_utils::greenprint "❌ Failed"
    exit 1
fi
