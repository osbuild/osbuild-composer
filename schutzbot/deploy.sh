#!/bin/bash
set -euxo pipefail

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

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

if [[ -n "${RHN_REGISTRATION_SCRIPT:-}" ]] && ! sudo subscription-manager status; then
    greenprint "Registering RHEL"
    sudo chmod +x "$RHN_REGISTRATION_SCRIPT"
    sudo "$RHN_REGISTRATION_SCRIPT"
fi

greenprint "Restarting systemd to work around some Fedora issues in cloud images"
sudo systemctl restart systemd-journald

greenprint "Removing Fedora's modular repositories to speed up dnf"
sudo rm -f /etc/yum.repos.d/fedora*modular*

greenprint "Enabling fastestmirror and disabling weak dependencies to speed up dnf even more 🏎️"
echo -e "fastestmirror=1\ninstall_weak_deps=0" | sudo tee -a /etc/dnf/dnf.conf

# Ensure we are using the latest dnf since early revisions of Fedora 31 had
# some dnf repo priority bugs like BZ 1733582.
# NOTE(mhayden): We can exclude kernel updates here to save time with dracut
# and module updates. The system will not be rebooted in CI anyway, so a
# kernel update is not needed.
if [[ $ID == fedora ]]; then
    greenprint "Upgrading system to fix dnf issues"
    sudo dnf -y upgrade --exclude kernel --exclude kernel-core
fi

greenprint "Adding osbuild team ssh keys"
cat schutzbot/team_ssh_keys.txt | tee -a ~/.ssh/authorized_keys > /dev/null

greenprint "Setting up a dnf repository for the RPMs we built via mock"
sudo cp osbuild-mock.repo /etc/yum.repos.d/osbuild-mock.repo
sudo dnf repository-packages osbuild-mock list

greenprint "Installing the Image Builder packages"
# Note: installing only -tests to catch missing dependencies
# TODO: remove make to fulfill the previous line's statement
retry sudo dnf -y install osbuild-composer-tests make

greenprint "Setting up a directory to hold repository overrides"
sudo mkdir -p /etc/osbuild-composer/repositories

if [[ -f "rhel-8.json" ]]; then
    greenprint "Overriding default osbuild-composer rhel-8 sources"
    sudo cp rhel-8.json /etc/osbuild-composer/repositories/
fi

if [[ -f "rhel-8-beta.json" ]]; then
    greenprint "Overriding default osbuild-composer rhel-8-beta sources"
    sudo cp rhel-8-beta.json /etc/osbuild-composer/repositories/
fi

greenprint "Copying custom composer config"
# Remote worker needs this
sudo mkdir -p /etc/osbuild-composer
sudo cp test/image-tests/osbuild-composer.toml \
    /etc/osbuild-composer/

greenprint "Generating SSL certificates"
sudo make composer-key-pair
sudo make worker-key-pair

greenprint "Starting services"
sudo systemctl enable --now osbuild-remote-worker.socket
sudo systemctl enable --now osbuild-composer.socket

if rpm -q osbuild-composer-koji; then
    sudo systemctl enable --now osbuild-composer-koji.socket
fi

if [[ $ID == rhel ]]; then
    greenprint "Starting cloud socket"
    sudo systemctl enable --now osbuild-composer-cloud.socket
fi

greenprint "Verifying that the API is running"
sudo composer-cli status show
sudo composer-cli sources list
for SOURCE in $(sudo composer-cli sources list); do
    sudo composer-cli sources info "$SOURCE"
done
