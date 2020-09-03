#!/bin/bash
set -euox pipefail

CHECKOUT_DIRECTORY="${PWD}"
WORKING_DIRECTORY=/usr/libexec/osbuild-composer
TESTS_PATH=/usr/libexec/tests/osbuild-composer

PASSED_TESTS=()
FAILED_TESTS=()

TEST_RUNNER=/usr/libexec/tests/osbuild-composer/osbuild-base-test-runner

function retry {
    local count=0
    local retries=5
    until "$@"; do
        exit=$?
        count=$(($count + 1))
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

# Register RHEL if we are provided with a registration script.
if [[ -n "${RHN_REGISTRATION_SCRIPT:-}" ]] && ! sudo subscription-manager status; then
    sudo chmod +x $RHN_REGISTRATION_SCRIPT
    sudo $RHN_REGISTRATION_SCRIPT
fi

# Restart systemd to work around some Fedora issues in cloud images.
sudo systemctl restart systemd-journald

# Remove Fedora's modular repositories to speed up dnf.
sudo rm -f /etc/yum.repos.d/fedora*modular*

# Enable fastestmirror and disable weak dependency installation to speed up
# dnf operations.
echo -e "fastestmirror=1\ninstall_weak_deps=0" | sudo tee -a /etc/dnf/dnf.conf

# Ensure we are using the latest dnf since early revisions of Fedora 31 had
# some dnf repo priority bugs like BZ 1733582.
# NOTE(mhayden): We can exclude kernel updates here to save time with dracut
# and module updates. The system will not be rebooted in CI anyway, so a
# kernel update is not needed.
if [[ $ID == fedora ]]; then
    sudo dnf -y upgrade --exclude kernel --exclude kernel-core
fi

# Add osbuild team ssh keys.
cat schutzbot/team_ssh_keys.txt | tee -a ~/.ssh/authorized_keys > /dev/null

# Set up a dnf repository for the RPMs we built via mock.
sudo cp osbuild-mock.repo /etc/yum.repos.d/osbuild-mock.repo
sudo dnf repository-packages osbuild-mock list

# Ensure osbuild-composer-tests is installed.
retry sudo dnf -y install osbuild-composer-tests

ls -l /usr/libexec/tests/osbuild-composer

# Change to the working directory.
cd $WORKING_DIRECTORY

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
    if sudo --preserve-env $TEST_RUNNER $1; then
        PASSED_TESTS+=$1
    else
        FAILED_TESTS+=$1
    fi
    test_divider
    echo
}

# Run
run_test_case /usr/libexec/tests/osbuild-composer/osbuild-weldr-tests
run_test_case /usr/libexec/tests/osbuild-composer/osbuild-tests
sudo --preserve-env $TEST_RUNNER -cleanup

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