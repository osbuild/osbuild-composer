#!/bin/bash
set -euxo pipefail

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

# Install the Image Builder packages.
# Note: installing only -tests to catch missing dependencies
retry sudo dnf -y install osbuild-composer-tests make

# Set up a directory to hold repository overrides.
sudo mkdir -p /etc/osbuild-composer/repositories

# Override default osbuild-composer sources if local definition exists
if [[ -f "rhel-8.json" ]]; then
    sudo cp rhel-8.json /etc/osbuild-composer/repositories/
fi

if [[ -f "rhel-8-beta.json" ]]; then
    sudo cp rhel-8-beta.json /etc/osbuild-composer/repositories/
fi

# Generate SSL certificates
sudo make composer-key-pair
sudo make worker-key-pair

# Start services.
sudo systemctl enable --now osbuild-composer.socket
sudo systemctl enable --now osbuild-composer-koji.socket

# Verify that the API is running.
sudo composer-cli status show
sudo composer-cli sources list
for SOURCE in `sudo composer-cli sources list`; do
    sudo composer-cli sources info $SOURCE
done
