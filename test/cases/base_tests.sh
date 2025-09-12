#!/bin/bash
set -euo pipefail

#
# Script that executes different high level sanity tests writen in GO. 
#
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

WORKING_DIRECTORY=/usr/libexec/osbuild-composer
TESTS_PATH=/usr/libexec/osbuild-composer-test
mkdir --parents /tmp/logs
LOGS_DIRECTORY=$(mktemp --directory --tmpdir=/tmp/logs)

PASSED_TESTS=()
FAILED_TESTS=()

TEST_CASES_ON_PREM=(
  "osbuild-weldr-tests"
  "osbuild-composer-cli-tests"
)

TEST_CASES_SERVICE=(
  "osbuild-auth-tests"
)

# Print out a nice test divider so we know when tests stop and start.
test_divider () {
    printf "%0.s-" {1..78} && echo
}

# Run a test case and store the result as passed or failed.
run_test_case () {
    TEST_NAME=$(basename "$1")
    echo
    test_divider
    echo "🏃🏻 Running test: ${TEST_NAME}"
    test_divider

    if sudo "${1}" -test.v | tee "${LOGS_DIRECTORY}"/"${TEST_NAME}".log; then
        PASSED_TESTS+=("$TEST_NAME")
    else
        FAILED_TESTS+=("$TEST_NAME")
    fi

    test_divider
    echo
}


# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Explicitly install osbuild-depsolve-dnf in nightly pipeline until the
# osbuild-composer version which has the dependency lands there (v101).
if ! nvrGreaterOrEqual "osbuild-composer" "101"; then
    echo "osbuild-composer version is too old, explicitly installing osbuild-depsolve-dnf"
    sudo dnf install -y osbuild-depsolve-dnf
fi

# Change to the working directory.
cd $WORKING_DIRECTORY

# Run each test case.
for TEST_CASE in "${TEST_CASES_ON_PREM[@]}"; do
    run_test_case ${TESTS_PATH}/"$TEST_CASE"
done

/usr/libexec/osbuild-composer-test/provision.sh tls

# Run each test case.
for TEST_CASE in "${TEST_CASES_SERVICE[@]}"; do
    run_test_case ${TESTS_PATH}/"$TEST_CASE"
done

# Print a report of the test results.
test_divider
greenprint "😃 Passed tests:" "${PASSED_TESTS[@]}"
redprint "☹ Failed tests:" "${FAILED_TESTS[@]}"
test_divider

# Exit with a failure if tests were executed and any of them failed.
if [ ${#PASSED_TESTS[@]} -gt 0 ] && [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    greenprint "🎉 All tests passed."
    exit 0
else
    redprint "🔥 One or more tests failed."
    exit 1
fi
