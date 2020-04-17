#!/bin/bash
set -euxo pipefail

source /etc/os-release

get_fastest_mirror() {
  FEDORA_VERSION=${1:-31}
  curl -s "https://mirrors.fedoraproject.org/metalink?repo=fedora-${FEDORA_VERSION}&arch=x86_64" | \
    xpath -e "/metalink/files/file/resources/url[@protocol='http']/text()" 2>/dev/null | \
    head -n 1 | \
    sed 's#releases.*##' || true
}

# Install packages.
sudo dnf -y install ansible perl-XML-XPath

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

# Modify the URLs in the test cases to use a nearby mirror.
TEST_CASE_DIR=/usr/share/tests/osbuild-composer
FASTEST_MIRROR=$(get_fastest_mirror)
sed -i "s#http://dl.fedoraproject.org/pub/fedora/linux/releases/#${FASTEST_MIRROR}releases/#" ${TEST_CASE_DIR}/cases/*.json

# Test wrapper that automatically collects logs.
run_osbuild_test() {
  TEST_NAME=$1
  TEST_CASE=${2:-}

  # Log each test case individually, if provided.
  if  [[ -z $TEST_CASE ]]; then
    LOGFILE=${TEST_NAME}.log
  else
    # Strip the test case name down to just the filename without extension.
    FILENAME=$(basename $TEST_CASE)
    LOGFILE=${TEST_NAME}-${FILENAME%%.*}.log
  fi

  /usr/libexec/tests/osbuild-composer/${TEST_NAME} \
    -test.v $TEST_CASE 2>&1 | tee $LOGFILE > /dev/null &
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

# Run the Fedora 31 image tests on Fedora 31 only.
if [[ $NAME == 'Fedora' ]] && [[ $VERSION_ID == "31" ]]; then
  run_osbuild_test osbuild-image-tests \
    ${TEST_CASE_DIR}/cases/fedora_31-x86_64-qcow2-boot.json
  wait

  run_osbuild_test osbuild-image-tests \
    ${TEST_CASE_DIR}/cases/fedora_31-x86_64-tar-boot.json
  wait
fi