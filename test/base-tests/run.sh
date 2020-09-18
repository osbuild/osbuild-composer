#!/bin/bash
set -euo pipefail

WORKING_DIRECTORY="/usr/libexec/osbuild-composer"
TEST_RUNNER="/usr/libexec/tests/osbuild-composer/osbuild-base-test-runner"

function retry {
    local count=0
    local retries=5
    until "$@"; do
        exit=$?
        count=$((count + 1))
        if [[ $count -lt $retries ]]; then
            echo "Retrying command..."
            sleep 1
        else
            echo "Command failed after ${retries} retries. Giving up."
            return $exit
        fi
    done
    return 0
}

# Get OS details.
source /etc/os-release

# Add osbuild team ssh keys.
cat schutzbot/team_ssh_keys.txt >> ~/.ssh/authorized_keys

# Set up a dnf repository for the RPMs we built via mock.
sudo cp osbuild-mock.repo /etc/yum.repos.d/osbuild-mock.repo
sudo dnf repository-packages osbuild-mock list

# Ensure osbuild-composer-tests is installed.
retry sudo dnf -y install osbuild-composer-tests

ls -l /usr/libexec/tests/osbuild-composer

# Change to the working directory.
cd "${WORKING_DIRECTORY}"

PASSED_TESTS=()
FAILED_TESTS=()

# Print out a nice test divider so we know when tests stop and start.
test_divider () {
    printf "%0.s-" {1..78} && echo
}

run_test_case () {
    test_divider
    echo "🏃🏻 Running test: $1"
    # In case the socket exists, remove it
    sudo rm -f /run/weldr/api.socket
    if sudo --preserve-env "${TEST_RUNNER}" "$1"; then
        PASSED_TESTS+=("$1")
    else
        FAILED_TESTS+=("$1")
    fi
    test_divider
    echo
}

# Run
run_test_case /usr/libexec/tests/osbuild-composer/osbuild-weldr-tests
run_test_case /usr/libexec/tests/osbuild-composer/osbuild-tests
sudo --preserve-env "${TEST_RUNNER}" -cleanup

# Print a report of the test results.
test_divider
echo "😃 Passed tests:" "${PASSED_TESTS[@]}"
echo "☹ Failed tests:" "${FAILED_TESTS[@]}"
test_divider

# Exit with a failure if any tests failed.
if [ ${#FAILED_TESTS[@]} -eq 0 ]; then
    echo "🎉 All tests passed."
    exit 0
else
    echo "🔥 One or more tests failed."
    exit 1
fi