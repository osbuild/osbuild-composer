#!/bin/bash
set -euxo pipefail

# Get OS details.
source /etc/os-release

# Restart systemd to work around some Fedora issues in cloud images.
sudo systemctl restart systemd-journald

# Add osbuild team ssh keys.
cat schutzbot/team_ssh_keys.txt | tee -a ~/.ssh/authorized_keys > /dev/null

# Set up a dnf repository for the RPMs we built via mock.
sudo cp osbuild-mock.repo /etc/yum.repos.d/osbuild-mock.repo
sudo dnf repository-packages osbuild-mock list

# Install the Image Builder packages.
sudo dnf -y install composer-cli osbuild osbuild-ostree \
    osbuild-composer osbuild-composer-rcm osbuild-composer-tests \
    osbuild-composer-worker python3-osbuild

# Start services.
sudo systemctl enable --now osbuild-rcm.socket
sudo systemctl enable --now osbuild-composer.socket

# Verify that the API is running.
sudo composer-cli status show
sudo composer-cli sources list