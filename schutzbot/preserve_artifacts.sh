#!/bin/bash
set -euxo pipefail

# Get details about the OS.
source /etc/os-release

# Set a distro label.
DISTRO=${ID}-${VERSION_ID//./}

echo "Preserving artifacts for:"
echo "  BUILD_NUMBER: ${BUILD_NUMBER}"
echo "  DISTRO:       ${DISTRO}"
echo "  TEST_TYPE:    ${TEST_TYPE}"

if [[ ! -x /usr/bin/rsync ]] || [[ ! -x /usr/bin/openstack ]]; then
    dnf -qy install rsync python3-openstackclient
fi

BASE_ARTIFACT_DIR=${WORKSPACE}/artifacts/${JOB_NAME}-${BUILD_NUMBER}

# RPMs only need to be preserved one time for a particular OS.
REPO_DIR=/tmp/mock_repo/repo
REPO_ARTIFACT_DIR=${BASE_ARTIFACT_DIR}/repo
mkdir -p $REPO_DIR
rsync -av $REPO_ARTIFACT_DIR/ $REPO_DIR/

# Logs must be unique across each OS and test type combination.
LOG_ARTIFACT_DIR=${BASE_ARTIFACT_DIR}/logs/${DISTRO}-${TEST_TYPE}
mkdir -p $LOG_ARTIFACT_DIR
cp -av *.log $LOG_ARTIFACT_DIR

# Set up OpenStack credentials.
mkdir ~/.config/openstack
cp $OPENSTACK_CREDS ~/.config/openstack/clouds.yaml

# Upload artifacts to OpenStack Swift.
pushd ${WORKSPACE}/artifacts
    for filename in $(find . -type f); do
        # Try up to 5 times just in case there are API errors.
        for i in {1..5}; do
            openstack --os-cloud=psi object create \
                --name ${filename} ${OPENSTACK_CONTAINER} ${filename}
            if [[ $? == 0 ]]; then
                break
            fi
        fi
    done
popd

# Clean up OpenStack credentials.
rm -f ~/.config/openstack/clouds.yaml