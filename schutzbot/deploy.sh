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
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/${project}/${DISTRO_VERSION}/${ARCH}/${commit}
enabled=1
gpgcheck=0
priority=${priority}
EOF
}

# Get OS details.
source tools/set-env-variables.sh

# Distro version that this script is running on.
DISTRO_VERSION=${ID}-${VERSION_ID}

if [[ "$ID" == rhel ]] && sudo subscription-manager status; then
  # If this script runs on subscribed RHEL, install content built using CDN
  # repositories.
  DISTRO_VERSION=rhel-${VERSION_ID%.*}-cdn
fi

greenprint "Enabling fastestmirror to speed up dnf 🏎️"
echo -e "fastestmirror=1" | sudo tee -a /etc/dnf/dnf.conf

greenprint "Adding osbuild team ssh keys"
cat schutzbot/team_ssh_keys.txt | tee -a ~/.ssh/authorized_keys > /dev/null

# TODO: include this in the jenkins runner (and split test/target machines out)
sudo dnf -y install jq

# fallback for gitlab
GIT_COMMIT="${GIT_COMMIT:-${CI_COMMIT_SHA}}"

setup_repo osbuild-composer "${GIT_COMMIT}" 5

OSBUILD_GIT_COMMIT=$(cat Schutzfile | jq -r '.["'"${ID}-${VERSION_ID}"'"].dependencies.osbuild.commit')
if [[ "${OSBUILD_GIT_COMMIT}" != "null" ]]; then
  setup_repo osbuild "${OSBUILD_GIT_COMMIT}" 10
fi

if [[ "$PROJECT" != "osbuild-composer" ]]; then
  PROJECT_COMMIT=$(jq -r ".[\"${ID}-${VERSION_ID}\"].dependants[\"${PROJECT}\"].commit" Schutzfile)
  setup_repo "${PROJECT}" "${PROJECT_COMMIT}" 10

  # Get a list of packages needed to be preinstalled before "${PROJECT}-tests".
  # Useful mainly for EPEL.
  PRE_INSTALL_PACKAGES=$(jq -r ".[\"${ID}-${VERSION_ID}\"].dependants[\"${PROJECT}\"].pre_install_packages[]?" Schutzfile)

  if [ "${PRE_INSTALL_PACKAGES}" ]; then
    # shellcheck disable=SC2086 # We need to pass multiple arguments here.
    sudo dnf -y install ${PRE_INSTALL_PACKAGES}
  fi
fi

if [ -f "rhel8internal.repo" ]; then
    greenprint "Preparing repos for internal build testing"
    sudo mv rhel8internal.repo /etc/yum.repos.d/
    # Use osbuild from schutzfile if desired for testing custom osbuild-composer packages
    # specified by $REPO_URL in ENV and used in prepare-rhel-internal.sh
    if [ "$SCHUTZ_OSBUILD" == 1 ]; then
        sudo rm -f /etc/yum.repos.d/osbuild-composer.repo
    else
        sudo rm -f /etc/yum.repos.d/osbuild*.repo
    fi
fi

greenprint "Installing test packages for ${PROJECT}"
# Note: installing only -tests to catch missing dependencies
retry sudo dnf -y install "${PROJECT}-tests"

if [ -n "${CI}" ]; then
    # copy repo files b/c GitLab can't upload artifacts
    # which are outside the build directory
    cp /etc/yum.repos.d/*.repo "$(pwd)"
fi
