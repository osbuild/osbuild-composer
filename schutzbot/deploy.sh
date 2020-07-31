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
    # TODO: copy this to the target VM
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
    # There is a race condition in dnf where an RPM package is present in the
    # cache when the transaction starts, but it is missing when it is to be
    # installed. Workaround this issue by sleeping for 2 minutes.
    sleep 120
    sudo dnf clean all
    sudo dnf -y upgrade --exclude kernel --exclude kernel-core
fi

if [ "${1-""}" != "composer" ];
then
    # Add osbuild team ssh keys.
    cat schutzbot/team_ssh_keys.txt | tee -a ~/.ssh/authorized_keys > /dev/null
fi

# Set up a dnf repository for the RPMs we built via mock.
if [ "${1-""}" == "composer" ];
then
    sudo cp /tmp/osbuild-mock.repo /etc/yum.repos.d/osbuild-mock.repo
else
    sudo cp osbuild-mock.repo /etc/yum.repos.d/osbuild-mock.repo
fi
sudo dnf repository-packages osbuild-mock list

# Install the Image Builder packages.
if [ "${1-""}" == "composer" ];
then
    # This is the target image. Install only osbuild-composer.
    retry sudo dnf -y install osbuild-composer
else
    # Note: installing only -tests to catch missing dependencies
    retry sudo dnf -y install osbuild-composer-tests
fi


# Set up a directory to hold repository overrides.
sudo mkdir -p /etc/osbuild-composer/repositories

# NOTE(mhayden): RHEL 8.3 is the release we are currently targeting, but the
# release is in beta right now. For RHEL 8.2 (latest release), ensure that
# the production (non-beta content) is used.
if [[ "${ID}${VERSION_ID//./}" == rhel82 ]]; then
    # TODO: copy this into the target VM
    sudo cp ${WORKSPACE}/test/external-repos/rhel-8.json \
        /etc/osbuild-composer/repositories/rhel-8.json
fi

if [ "${1-""}" == "composer" ];
then
    # Start services.
    sudo systemctl enable --now osbuild-composer.socket
elif [ "${1-""}" == "tests" ];
then
    # Verify that the API is running.
    sudo composer-cli status show
    sudo composer-cli sources list
else
    # Start services.
    sudo systemctl enable --now osbuild-composer.socket
    # Verify that the API is running.
    sudo composer-cli status show
    sudo composer-cli sources list
fi
