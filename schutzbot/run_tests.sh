#!/bin/bash
set -euxo pipefail

# Create temporary directories for Ansible.
sudo mkdir -vp /opt/ansible_{local,remote}
sudo chmod -R 777 /opt/ansible_{local,remote}

# Restart systemd to work around some Fedora issues in cloud images.
sudo systemctl restart systemd-journald

# Get the current journald cursor.
export JOURNALD_CURSOR=$(sudo journalctl --quiet -n 1 --show-cursor | tail -n 1 | grep -oP 's\=.*$')

# Add a function to preserve the system journal if something goes wrong.
preserve_journal() {
  sudo journalctl --after-cursor=${JOURNALD_CURSOR} > systemd-journald.log
  exit 1
}
trap "preserve_journal" ERR

# Write a simple hosts file for Ansible.
echo -e "[test_instances]\nlocalhost ansible_connection=local" > hosts.ini

# Set Ansible's config file location.
export ANSIBLE_CONFIG=ansible-osbuild/ansible.cfg

# Get the SHA of osbuild-composer which Jenkins checked out for us.
OSBUILD_COMPOSER_VERSION=$(git rev-parse HEAD)

# Deploy osbuild-composer and osbuild using RPMs built in a mock chroot.
git clone https://github.com/osbuild/ansible-osbuild.git ansible-osbuild
ansible-playbook \
  -i hosts.ini \
  -e osbuild_composer_version=${OSBUILD_COMPOSER_VERSION} \
  -e install_source=mock \
  ansible-osbuild/playbook.yml

# Run the tests.
ansible-playbook \
  -e workspace=${WORKSPACE} \
  -e journald_cursor="${JOURNALD_CURSOR}" \
  -e test_type=${TEST_TYPE:-base} \
  -i hosts.ini \
  schutzbot/test.yml

# Collect the systemd journal anyway if we made it all the way to the end.
sudo journalctl --after-cursor=${JOURNALD_CURSOR} > systemd-journald.log