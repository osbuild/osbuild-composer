#!/bin/bash
set -euxo pipefail

# Restart systemd to work around some Fedora issues in cloud images.
systemctl restart systemd-journald

# Get the current journald cursor.
export JOURNALD_CURSOR=$(journalctl --quiet -n 1 --show-cursor | tail -n 1 | grep -oP 's\=.*$')

# Add a function to preserve the system journal if something goes wrong.
preserve_journal() {
  journalctl --after-cursor=${JOURNALD_CURSOR} > systemd-journald.log
  exit 1
}
trap "preserve_journal" ERR

# Ensure Ansible is installed.
if ! rpm -q ansible; then
  sudo dnf -y install ansible
fi

# Write a simple hosts file for Ansible.
echo -e "[test_instances]\nlocalhost ansible_connection=local" > hosts.ini

# Set Ansible's config file location.
export ANSIBLE_CONFIG=ansible-osbuild/ansible.cfg

# Deploy the software.
git clone https://github.com/osbuild/ansible-osbuild.git ansible-osbuild
ansible-playbook \
  -i hosts.ini \
  -e osbuild_composer_repo=${WORKSPACE} \
  -e osbuild_composer_version=$(git rev-parse HEAD) \
  ansible-osbuild/playbook.yml

# Run the tests.
ansible-playbook \
  -e workspace=${WORKSPACE} \
  -e journald_cursor="${JOURNALD_CURSOR}" \
  -e test_type=${TEST_TYPE:-base} \
  -i hosts.ini \
  jenkins/test.yml

# Collect the systemd journal anyway if we made it all the way to the end.
journalctl --after-cursor=${JOURNALD_CURSOR} > systemd-journald.log