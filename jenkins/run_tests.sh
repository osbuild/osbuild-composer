#!/bin/bash
set -euxo pipefail

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

# Run the integration tests.
/usr/libexec/tests/osbuild-composer/osbuild-dnf-json-tests
## Skipping the image tests because they take forever.
# /usr/libexec/tests/osbuild-composer/osbuild-image-tests
## Skipping because of a failure.
## See https://github.com/osbuild/osbuild-composer/pull/439#issuecomment-606866374.
#/usr/libexec/tests/osbuild-composer/osbuild-rcm-tests
/usr/libexec/tests/osbuild-composer/osbuild-tests
/usr/libexec/tests/osbuild-composer/osbuild-weldr-tests
