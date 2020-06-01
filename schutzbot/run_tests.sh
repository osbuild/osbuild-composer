#!/bin/bash
set -euxo pipefail

# Get OS details.
source /etc/os-release

# Set up a dnf repository for the RPMs we built via mock.
sudo cp osbuild-mock.repo /etc/yum.repos.d/osbuild-mock.repo
dnf repository-packages osbuild-mock list

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

# Deploy osbuild/osbuild-composer via the repository we created.
export ANSIBLE_CONFIG=ansible-osbuild/ansible.cfg
git clone https://github.com/osbuild/ansible-osbuild.git ansible-osbuild
ansible-playbook \
  -i hosts.ini \
  -e install_source=os \
  ansible-osbuild/playbook.yml

# Run the tests.
echo "The AWS_BUCKET env var is set to: ${AWS_BUCKET:-}"
ansible-playbook \
  -e workspace=${WORKSPACE} \
  -e test_type=${TEST_TYPE:-base} \
  -i hosts.ini \
  schutzbot/test.yml

# Collect the systemd journal anyway if we made it all the way to the end.
sudo journalctl --after-cursor=${JOURNALD_CURSOR} > systemd-journald.log
