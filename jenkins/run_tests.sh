#!/bin/bash
set -euxo pipefail

echo "WOOT"

# Ensure Ansible is installed.
sudo dnf -y install ansible

# Clone the latest version of ansible-osbuild.
git clone https://github.com/osbuild/ansible-osbuild.git ansible-osbuild

# Get the current SHA of osbuild-composer.
OSBUILD_COMPOSER_VERSION=$(git rev-parse HEAD)

# Run the deployment.
pushd ansible-osbuild
  echo -e "[test_instances]\nlocalhost ansible_connection=local" > hosts.ini
  ansible-playbook \
    -i hosts.ini \
    -e osbuild_composer_repo=${WORKSPACE} \
    -e osbuild_composer_version=${OSBUILD_COMPOSER_VERSION} \
    playbook.yml
popd

run_osbuild_test() {
  TEST_NAME=$1
  /usr/libexec/tests/osbuild-composer/${TEST_NAME} \
    -test.v | tee ${TEST_NAME}.log > /dev/null &
}

# Run the rcm and weldr tests separately to avoid API errors and timeouts.
run_osbuild_test osbuild-rcm-tests
run_osbuild_test osbuild-weldr-tests

# Run the dnf and other tests together.
TEST_PIDS=()
run_osbuild_test osbuild-tests
TEST_PIDS+=($!)
run_osbuild_test osbuild-dnf-json-tests
TEST_PIDS+=($!)
for TEST_PID in "${TEST_PIDS[@]}"; do
  wait $TEST_PID
done

# Wait on the image tests for now.
# /usr/libexec/tests/osbuild-composer/osbuild-image-tests
