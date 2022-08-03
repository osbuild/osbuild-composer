#!/bin/bash
set -euo pipefail

MANIFEST_TESTS_RUNNER="/usr/libexec/osbuild-composer-test/osbuild-composer-manifest-tests"
DNF_JSON_PATH="/usr/libexec/osbuild-composer/dnf-json"
IMAGE_TEST_CASES_PATH="/usr/share/tests/osbuild-composer/manifests"

WORKING_DIRECTORY=/usr/libexec/osbuild-composer
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Print out a nice test divider so we know when tests stop and start.
test_divider () {
    printf "%0.s-" {1..78} && echo
}

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Change to the working directory.
cd $WORKING_DIRECTORY

# Run test case.
TEST_NAME=$(basename "$MANIFEST_TESTS_RUNNER")
echo
test_divider
echo "🏃🏻 Running test: ${TEST_NAME}"
test_divider

if sudo "$MANIFEST_TESTS_RUNNER" -test.v -manifests-path "$IMAGE_TEST_CASES_PATH" -dnf-json-path "$DNF_JSON_PATH" | tee "${ARTIFACTS}"/"${TEST_NAME}".log; then
    echo "🎉  Test passed."
    exit 0
else
    echo "🔥 Test failed."
    exit 1
fi
