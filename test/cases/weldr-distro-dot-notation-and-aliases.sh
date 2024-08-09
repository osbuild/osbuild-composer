#!/bin/bash

# Test case for Weldr API (on-premise) distro dot notation and aliases.
#
# This test case verifies that distributions can be specified with and without
# the dot to sepratae the major and minor version (true for RHEL-8 and RHEL-9).
# It also verifies the behavior of distro name aliases
# (e.g. "rhel-9" -> "rhel-9.5"). This is done by building a SAP image and
# inspecting it using guestfish. Specifically, the SAP image contains DNF VAR
# config /etc/dnf/vars/releasever, which contains value "X.Y", which should be
# the same as the distro release that the alias points to.

set -xeuo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Don't run in nightly pipeline until we have the version with dot-notation in nightly compose
if ! nvrGreaterOrEqual "osbuild-composer" "100"; then
    echo "osbuild-composer version is too old, skipping the test"
    exit 0
fi

TMPDIR=$(mktemp -d)
greenprint "Registering clean ups"
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    set +eu
    rm -rf "${TMPDIR}"
    set -eu
}
trap cleanup EXIT

# Remove any restrictions on the image types for weldr API, since
# testing the distro alias requires building the SAP image.
#
# Also override the alias for RHEL 8 since the default alias, 8.10, doesn't
# have a locked version, so the releasever is not set in the dnf vars, and this
# test relies on that file to verify that we are using the correct distro
# object and code path.
EXTRA_COMPOSER_CONF="$(mktemp -p "$TMPDIR")"
cat <<EOF | tee "${EXTRA_COMPOSER_CONF}"
# overrides the default rhel-* configuration
[weldr_api.distros."rhel-*"]

# overrides the default rhel-8 alias
[distro_aliases]
rhel-8 = "rhel-8.8"
rhel-9 = "rhel-9.5"
EOF

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none "${EXTRA_COMPOSER_CONF}"

# install test dependencies
sudo dnf install -y '/usr/bin/guestfish'

# path for repository overrides with higher priority
REPO_OVERRIDES_DIR=/etc/osbuild-composer/repositories
# path for repositories shipped with osbuild-composer (lower priority)
REPO_SHIPPED_DIR="/usr/share/osbuild-composer/repositories"
# path for repositories shipped with osbuild-composer tests, intended for
# use as part of tests.
REPO_TEST_DIR="/usr/share/tests/osbuild-composer/repositories"

DISTRO="${ID}-${VERSION_ID}"
DISTRO_WITHOUT_DOT="${ID}-${VERSION_ID//./}"
DISTRO_WITHOUT_MINOR="${ID}-${VERSION_ID%.*}"

DISTRO_TEST_REPO="${REPO_TEST_DIR}/${DISTRO}.json"

# Clean up the repository directories
function clean_repo_dirs() {
    sudo rm -rf "${REPO_OVERRIDES_DIR}"
    sudo mkdir -p "${REPO_OVERRIDES_DIR}"
    sudo rm -rf "${REPO_SHIPPED_DIR}"
    sudo mkdir -p "${REPO_SHIPPED_DIR}"
}

# Stop osbuild-composer, worker and all sockets
function stop_osbuild_composer() {
    sudo systemctl stop 'osbuild*'
}

# Start osbuild-composer with clean cache and store
function start_osbuild_composer_clean() {
    stop_osbuild_composer
    sudo rm -rf /var/cache/osbuild-composer
    sudo rm -rf /var/lib/osbuild-composer
    sudo systemctl start osbuild-composer.socket

    # check that the API is running
    sudo composer-cli status show
}

# Common test case for verifying that depsolving the provided blueprint works.
function _test_depsolving_bp() {
    if [[ $# -ne 2 ]]; then
        echo "Usage: _test_depsolving_bp <blueprint> <blueprint_name>"
        exit 1
    fi

    local blueprint="$1"
    local blueprint_name="$2"

    sudo composer-cli blueprints push "${blueprint}"
    sudo composer-cli blueprints depsolve "${blueprint_name}"
    sudo composer-cli blueprints delete "${blueprint_name}"
}

# Common test case for verifying that the provided blueprint can produce an image.
function _test_compose_bp() {
    if [[ $# -lt 3 || $# -gt 5 ]]; then
        echo "Usage: _test_compose_bp <blueprint> <blueprint_name> <tmpdir> [<image_type>] [<test_distro_alias>]"
        exit 1
    fi

    local blueprint="$1"
    local blueprint_name="$2"
    local tmpdir="$3"
    local image_type="${4:-qcow2}"
    local test_distro_alias="${5:-0}"

    local composestart="${tmpdir}/compose-start.json"
    local composeinfo="${tmpdir}/compose-info.json"

    sudo composer-cli blueprints push "${blueprint}"
    sudo composer-cli --json compose start "${blueprint_name}" "${image_type}" | tee "${composestart}"

    local composeid
    composeid=$(get_build_info '.build_id' "${composestart}")

    # Wait for the compose to finish.
    local composestatus
    echo "⏱ Waiting for compose to finish: ${composeid}"
    while true; do
        sudo composer-cli --json compose info "${composeid}" | tee "${composeinfo}" > /dev/null
        composestatus=$(get_build_info '.queue_status' "${composeinfo}")

        # Is the compose finished?
        if [[ ${composestatus} != RUNNING ]] && [[ ${composestatus} != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 30
    done

    jq . "${composeinfo}"

    # Did the compose finish with success?
    if [[ $composestatus != FINISHED ]]; then
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi

    if [[ "${test_distro_alias}" == "1" ]]; then
        pushd "${tmpdir}"
        _verify_distro_alias_img "${composeid}"
        popd
    fi

    sudo composer-cli compose delete "${composeid}" >/dev/null
}

# Verify that the image contains /etc/dnf/vars/releasever with the expected
# content. Specifically, verify that the content of /etc/dnf/vars/releasever
# is the same as the VERSION_ID of the distro that the distro alias point to.
function _verify_distro_alias_img() {
    if [[ $# -ne 1 ]]; then
        echo "Usage: _verify_distro_alias_img <compose_id>"
        exit 1
    fi

    local compose_id="$1"

    local image_file
    image_file=$(sudo composer-cli compose image "${compose_id}")

    if [[ ! -f "${image_file}" ]]; then
        echo "Image file ${image_file} does not exist."
        exit 1
    fi

    # uncompress the file if compressed
    if [[ "${image_file}" == *.xz ]]; then
        sudo unxz "${image_file}"
        image_file="${image_file%.xz}"
    fi

    local dnf_vars_releasever="/etc/dnf/vars/releasever"
    greenprint "Check ${dnf_vars_releasever} in the image"

    local dnf_vars_releasever_content
    dnf_vars_releasever_content=$(LIBGUESTFS_BACKEND=direct sudo --preserve-env=LIBGUESTFS_BACKEND guestfish --ro -a "${image_file}" -i cat "${dnf_vars_releasever}")

    if [[ "${dnf_vars_releasever_content}" != "${VERSION_ID}" ]]; then
        echo "Unexpected content of ${dnf_vars_releasever}: ${dnf_vars_releasever_content}"
        echo "Expected: ${VERSION_ID}"
        exit 1
    fi
}

# Create a testing blueprint with wget package.
function _create_blueprint() {
    if [[ $# -lt 2 ]] || [[ $# -gt 3 ]]; then
        echo "Usage: _create_blueprint <directory> <blueprint_name> [<distro>]"
        exit 1
    fi

    local directory="$1"
    local blueprint_name="$2"
    local distro="${3:-}"

    cat <<EOF > "${directory}/${blueprint_name}.toml"
name = "${blueprint_name}"
description = "A testing blueprint"
version = "0.0.1"
EOF

        if [[ -n "${distro}" ]]; then
            cat <<EOF >> "${directory}/${blueprint_name}.toml"
distro = "${distro}"
EOF
        fi

    cat <<EOF >> "${directory}/${blueprint_name}.toml"

[[packages]]
name = "wget"
EOF

    echo "${directory}/${blueprint_name}.toml"
}

# Common test case for repo configurations.
function _test_repo() {
    local test_name="$1"
    local distro="$2"

    greenprint "TEST: ${test_name}"

    local directory="${TMPDIR}/${test_name}"
    mkdir -p "${directory}"

    stop_osbuild_composer
    clean_repo_dirs
    sudo cp "${DISTRO_TEST_REPO}" "${REPO_SHIPPED_DIR}/${distro}.json"
    start_osbuild_composer_clean
    local test_bp
    test_bp=$(_create_blueprint "${directory}" "${test_name}")
    _test_depsolving_bp "${test_bp}" "${test_name}"
}

# Test that the repository definitions can contain a dot in its filename,
# separating the major and minor version.
function test_repo_with_dot() {
    _test_repo "repo_with_dot" "${DISTRO}"
}

# Test that the repository definitions can be used without a dot in its filename,
# even though the distro version may contain a dot.
function test_repo_without_dot() {
    _test_repo "repo_without_dot" "${DISTRO_WITHOUT_DOT}"
}

# Test that the repository definitions with and without a dot in its filename
# are equivalent and can override each other. Test uses empty file in the
# shipped directory and the test repository in the overrides directory to
# verify that the override happens.
function test_repo_dot_overrides() {
    local test_name="repo_dot_overrides"
    greenprint "TEST: ${test_name}"

    local directory="${TMPDIR}/${test_name}"
    mkdir -p "${directory}"

    # Case 1: repo without dot in shipped dir, repo with dot in overrides dir
    stop_osbuild_composer
    clean_repo_dirs
    echo "{}" | sudo tee "${REPO_SHIPPED_DIR}/${DISTRO_WITHOUT_DOT}.json"
    sudo cp "${DISTRO_TEST_REPO}" "${REPO_OVERRIDES_DIR}/${DISTRO}.json"
    start_osbuild_composer_clean
    local test_bp
    test_bp=$(_create_blueprint "${directory}" "${test_name}")
    _test_depsolving_bp "${test_bp}" "${test_name}"

    # Case 2: repo with dot in shipped dir, repo without dot in overrides dir
    stop_osbuild_composer
    clean_repo_dirs
    echo "{}" | sudo tee "${REPO_SHIPPED_DIR}/${DISTRO}.json"
    sudo cp "${DISTRO_TEST_REPO}" "${REPO_OVERRIDES_DIR}/${DISTRO_WITHOUT_DOT}.json"
    start_osbuild_composer_clean
    test_bp=$(_create_blueprint "${directory}" "${test_name}")
    _test_depsolving_bp "${test_bp}" "${test_name}"
}

# Common test case for distro names in BP.
function _test_distro() {
    local test_name="$1"
    local distro="$2"

    greenprint "TEST: ${test_name}"

    local directory="${TMPDIR}/${test_name}"
    mkdir -p "${directory}"

    stop_osbuild_composer
    clean_repo_dirs
    sudo cp "${DISTRO_TEST_REPO}" "${REPO_SHIPPED_DIR}"
    start_osbuild_composer_clean
    local test_bp
    test_bp=$(_create_blueprint "${directory}" "${test_name}" "${distro}")
    _test_depsolving_bp "${test_bp}" "${test_name}"
    _test_compose_bp "${test_bp}" "${test_name}" "${directory}"
}

# Test that the distro name in the blueprint can contain a dot in its name.
function test_distro_with_dot() {
    _test_distro "distro_with_dot" "${DISTRO}"
}

# Test that the distro name in the blueprint can be used without a dot in its name.
function test_distro_without_dot() {
    _test_distro "distro_without_dot" "${DISTRO_WITHOUT_DOT}"
}

# Test that the distro alias without the minor version can be used in the blueprint.
# This test case requires that the test case is run on RHEL version which is
# set as the target distro without minor version in the default composer configuration.
function test_distro_alias() {
    local test_name="distro_alias"
    greenprint "TEST: ${test_name}"

    if [[ "${ID}" != "rhel" ]]; then
        echo "Testing distro alias requires RHEL distro."
        exit 1
    fi

    local directory="${TMPDIR}/${test_name}"
    mkdir -p "${directory}"

    stop_osbuild_composer
    clean_repo_dirs
    sudo cp "${DISTRO_TEST_REPO}" "${REPO_SHIPPED_DIR}/${DISTRO_WITHOUT_MINOR}.json"
    start_osbuild_composer_clean
    local test_bp
    test_bp=$(_create_blueprint "${directory}" "${test_name}" "${DISTRO_WITHOUT_MINOR}")
    _test_depsolving_bp "${test_bp}" "${test_name}"
    _test_compose_bp "${test_bp}" "${test_name}" "${directory}" "ec2-sap" "1"
}

# Skip all tests if the DISTRO does not contain a dot.
if [[ "${DISTRO}" == "${DISTRO_WITHOUT_DOT}" ]]; then
    echo "The distro does not contain dot to separate major and minor version."
    echo "Skipping the test, since testing the dot notation or aliases does not make sense."
    exit 0
fi

# Run the tests.
test_repo_with_dot
test_repo_without_dot
test_repo_dot_overrides
test_distro_with_dot
test_distro_without_dot
test_distro_alias

# If we got here, all tests passed.
echo "All tests passed! 🥳"
exit 0
