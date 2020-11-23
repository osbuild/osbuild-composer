#!/bin/bash
set -euxo pipefail

# The project whose -tests package is installed.
#
# If it is osbuild-composer (the default), it is pulled from the same
# repository as the osbuild-composer under test. For all other projects, the
# "dependants" key in Schutzfile is consulted to determine the repository to
# pull the -test package from.
PROJECT=${1:-osbuild-composer}

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

function setup_repo {
  local project=$1
  local commit=$2
  local priority=${3:-10}
  greenprint "Setting up dnf repository for ${project} ${commit}"
  sudo tee "/etc/yum.repos.d/${project}.repo" << EOF
[${project}]
name=${project} ${commit}
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/${project}/${ID}-${VERSION_ID}/${ARCH}/${commit}
enabled=1
gpgcheck=0
priority=${priority}
EOF
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

# TODO: include this in the jenkins runner (and split test/target machines out)
sudo dnf -y install jq

setup_repo osbuild-composer "${GIT_COMMIT}" 5

OSBUILD_GIT_COMMIT=$(cat Schutzfile | jq -r '.["'"${ID}-${VERSION_ID}"'"].dependencies.osbuild.commit')
if [[ "${OSBUILD_GIT_COMMIT}" != "null" ]]; then
  setup_repo osbuild "${OSBUILD_GIT_COMMIT}" 10
fi

if [[ "$PROJECT" != "osbuild-composer" ]]; then
  PROJECT_COMMIT=$(jq -r ".[\"${ID}-${VERSION_ID}\"].dependants[\"${PROJECT}\"].commit" Schutzfile)
  setup_repo "${PROJECT}" "${PROJECT_COMMIT}" 10
fi

if [[ $ID == rhel ]]; then
    greenprint "Setting up EPEL repository"
    # we need this for ansible and koji
    sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
fi

greenprint "Installing test packages for ${PROJECT}"
# Note: installing only -tests to catch missing dependencies
retry sudo dnf -y install "${PROJECT}-tests"
