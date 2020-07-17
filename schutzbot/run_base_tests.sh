#!/bin/bash
set -euo pipefail

WORKING_DIRECTORY=/usr/libexec/osbuild-composer
TESTS_PATH=/usr/libexec/tests/osbuild-composer

PASSED_TESTS=()
FAILED_TESTS=()

TEST_CASES=(
  "osbuild-weldr-tests"
  "osbuild-dnf-json-tests"
  "osbuild-tests"
)

# Print out a nice test divider so we know when tests stop and start.
test_divider () {
    printf "%0.s-" {1..78} && echo
}

# Run a test case and store the result as passed or failed.
run_test_case () {
    TEST_NAME=$(basename $1)
    echo
    test_divider
    echo "🏃🏻 Running test: ${TEST_NAME}"
    test_divider

    if sudo ${1} -test.v | tee ${WORKSPACE}/${TEST_NAME}.log; then
        PASSED_TESTS+=($TEST_NAME)
    else
        FAILED_TESTS+=($TEST_NAME)
    fi

    test_divider
    echo
}

# Ensure osbuild-composer-tests is installed.
sudo dnf -y install osbuild-composer-tests

# Change to the working directory.
cd $WORKING_DIRECTORY

# Run each test case.
for TEST_CASE in "${TEST_CASES[@]}"; do
    run_test_case ${TESTS_PATH}/$TEST_CASE
done

# Print a report of the test results.
test_divider
echo "😃 Passed tests: ${PASSED_TESTS[@]}"
echo "☹ Failed tests: ${FAILED_TESTS[@]}"
test_divider

# Exit with a failure if any tests failed.
if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo "🎉 All tests passed."
    exit 0
else
    echo "🔥 One or more tests failed."
    exit 1
fi
