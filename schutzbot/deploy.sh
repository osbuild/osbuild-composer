#!/bin/bash
set -euxo pipefail

# The project whose -tests package is installed.
#
# If it is osbuild-composer (the default), it is pulled from the same
# repository as the osbuild-composer under test. For all other projects, the
# "dependants" key in Schutzfile is consulted to determine the repository to
# pull the -test package from.
PROJECT=${1:-osbuild-composer}

# set locale to en_US.UTF-8
sudo dnf install -y glibc-langpack-en
sudo localectl set-locale LANG=en_US.UTF-8

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
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

  local REPO_PATH=${project}/${DISTRO_VERSION}/${ARCH}/${commit}
  if [[ "${NIGHTLY:=false}" == "true" && "${project}" == "osbuild-composer" ]]; then
    REPO_PATH=nightly/${REPO_PATH}
  fi

  greenprint "Setting up dnf repository for ${project} ${commit}"
  sudo tee "/etc/yum.repos.d/${project}.repo" << EOF
[${project}]
name=${project} ${commit}
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/${REPO_PATH}
enabled=1
gpgcheck=0
priority=${priority}
EOF
}

# Get OS details.
source tools/set-env-variables.sh

if [[ $ID == "rhel" && ${VERSION_ID%.*} == "9" ]]; then
  # There's a bug in RHEL 9 that causes /tmp to be mounted on tmpfs.
  # Explicitly stop and mask the mount unit to prevent this.
  # Otherwise, the tests will randomly fail because we use /tmp quite a lot.
  # See https://bugzilla.redhat.com/show_bug.cgi?id=1959826
  greenprint "Disabling /tmp as tmpfs on RHEL 9"
  sudo systemctl stop tmp.mount && sudo systemctl mask tmp.mount
fi

if [[ $ID == "centos" && $VERSION_ID == "8" ]]; then
    # Workaround for https://bugzilla.redhat.com/show_bug.cgi?id=2065292
    # Remove when podman-4.0.2-2.el8 is in Centos 8 repositories
    greenprint "Updating libseccomp on Centos 8"
    sudo dnf upgrade -y libseccomp
fi

# Distro version that this script is running on.
DISTRO_VERSION=${ID}-${VERSION_ID}

if [[ "$ID" == rhel ]] && sudo subscription-manager status; then
  # If this script runs on subscribed RHEL, install content built using CDN
  # repositories.
  DISTRO_VERSION=rhel-${VERSION_ID%.*}-cdn

  # workaround for https://github.com/osbuild/osbuild/issues/717
  sudo subscription-manager config --rhsm.manage_repos=1
fi

greenprint "Enabling fastestmirror to speed up dnf 🏎️"
echo -e "fastestmirror=1" | sudo tee -a /etc/dnf/dnf.conf

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

if [ -f "rhel${VERSION_ID%.*}internal.repo" ]; then
    greenprint "Preparing repos for internal build testing"
    sudo mv rhel"${VERSION_ID%.*}"internal.repo /etc/yum.repos.d/
fi

greenprint "Installing test packages for ${PROJECT}"

# NOTE: WORKAROUND FOR DEPENDENCY BUG
retry sudo dnf -y upgrade selinux-policy

# Note: installing only -tests to catch missing dependencies
retry sudo dnf -y install "${PROJECT}-tests"

# Note: image-info is now part of osbuild-tools
retry sudo dnf -y install osbuild-tools

# Save osbuild-composer NVR to a file to be used as CI artifact
rpm -q osbuild-composer > COMPOSER_NVR

IMAGE_BUILDER_EXPERIMENTAL="${IMAGE_BUILDER_EXPERIMENTAL:-}"
if [[ "${IMAGE_BUILDER_EXPERIMENTAL}" != "" ]]; then
    greenprint "Adding experimental options to osbuild-composer.service"
    # Pass any experimental options into the systemd unit
    cat > experimental-override.conf << EOF
[Service]
Environment=IMAGE_BUILDER_EXPERIMENTAL="${IMAGE_BUILDER_EXPERIMENTAL}"
EOF

    cat experimental-override.conf | sudo systemctl edit --stdin osbuild-composer.service
    sudo systemctl daemon-reload
    sudo systemctl cat osbuild-composer.service

    if echo "${IMAGE_BUILDER_EXPERIMENTAL}" | grep -q "image-builder-manifest-generation=1"; then
        # TODO: configure dependency pinning for image-builder like we do for osbuild
        # https://issues.redhat.com/browse/HMS-9647
        greenprint "Installing image-builder for experimental manifest generation"
        sudo dnf -y install image-builder
    fi
fi


if [ "${NIGHTLY:=false}" == "true" ]; then
    # check if we've installed the osbuild-composer RPM from the nightly tree
    # under test or happen to install a newer version from one of the S3 repositories
    rpm -qi osbuild-composer
    if ! rpm -qi osbuild-composer | grep "Build Host" | grep "redhat.com"; then
        echo "ERROR: Installed osbuild-composer RPM is not the official one"
        exit 2
    else
        echo "INFO: Installed osbuild-composer RPM seems to be official"
    fi

    # cross-check the installed RPM against the one under COMPOSE_URL
    source tools/define-compose-url.sh

    INSTALLED=$(rpm -q --qf "%{name}-%{version}-%{release}.%{arch}.rpm" osbuild-composer)
    RPM_URL="${COMPOSE_URL}/compose/AppStream/${ARCH}/os/Packages/${INSTALLED}"
    RETURN_CODE=$(curl --silent -o -I -L -s -w "%{http_code}" "${RPM_URL}")
    if [ "$RETURN_CODE" != 200 ]; then
        echo "ERROR: Installed ${INSTALLED} not found at ${RPM_URL}. Response was ${RETURN_CODE}"
        exit 3
    else
        echo "INFO: Installed ${INSTALLED} found at ${RPM_URL}, which matches SUT!"
    fi
fi

if [ -n "${CI}" ]; then
    # copy repo files b/c GitLab can't upload artifacts
    # which are outside the build directory
    cp /etc/yum.repos.d/*.repo "$(pwd)"
fi

# NB: The following is a workaround for the issue that podman falls back to
# the 'cni' network backend when finding any container images in the local
# storage when executed for the first time. Since we started embedding
# container images in our CI runner images, this resulted in failures,
# because the OS is missing some required CNI plugins. Until we somehow fix
# this in osbuild, we explicitly set the network backend to 'netavark'.
# This is relevant only for RHEL-9 / c9s, because Fedora since F40 and el10
# support only `netavark` backend.
if [[ ($ID == "rhel" || $ID == "centos") && ${VERSION_ID%.*} == "9" ]]; then
    greenprint "containers.conf: explicitly setting network_backend to 'netavark'"
    sudo mkdir -p /etc/containers/containers.conf.d
    sudo tee /etc/containers/containers.conf.d/network_backend.conf > /dev/null << EOF
[network]
network_backend = "netavark"
EOF
fi
