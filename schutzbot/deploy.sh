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
ARCH=$(uname -m)

if [[ -n "${RHN_REGISTRATION_SCRIPT:-}" ]] && ! sudo subscription-manager status; then
    greenprint "Registering RHEL"
    sudo chmod +x "$RHN_REGISTRATION_SCRIPT"
    sudo "$RHN_REGISTRATION_SCRIPT"
fi

greenprint "Enabling fastestmirror to speed up dnf 🏎️"
echo -e "fastestmirror=1" | sudo tee -a /etc/dnf/dnf.conf

greenprint "Adding osbuild team ssh keys"
cat schutzbot/team_ssh_keys.txt | tee -a ~/.ssh/authorized_keys > /dev/null

greenprint "Setting up a dnf repository with the RPMs we want to test"
sudo tee /etc/yum.repos.d/osbuild-composer.repo << EOF
[osbuild-composer]
name=osbuild composer ${GIT_COMMIT}
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild-composer/${ID}-${VERSION_ID}/${ARCH}/${GIT_COMMIT}
enabled=1
gpgcheck=0
# Default dnf repo priority is 99. Lower number means higher priority.
priority=5
EOF

if [[ $ID == rhel ]]; then
    greenprint "Setting up EPEL repository"
    # we need this for ansible and koji
    sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
fi

greenprint "Installing the Image Builder packages"
# Note: installing only -tests to catch missing dependencies
retry sudo dnf -y install osbuild-composer-tests
