#!/bin/bash
set -euxo pipefail

# Get OS details.
source /etc/os-release

# Set variables.
CONTAINER=osbuildci-artifacts
WORKSPACE=${WORKSPACE:-$(pwd)}
MOCK_CONFIG="${ID}-${VERSION_ID%.*}-$(uname -m)"
REPO_DIR=repo/${BUILD_TAG}/${ID}${VERSION_ID//./}

# Build source RPMs.
make srpm
make -C osbuild srpm

# Fix RHEL 8 mock template for non-subscribed images.
if [[ $NODE_NAME == "rhel82*" ]] || [[ $NODE_NAME == "rhel83*" ]]; then
    sudo curl --retry 5 -Lsko /etc/mock/templates/rhel-8.tpl \
        https://gitlab.cee.redhat.com/snippets/2208/raw
fi

# Compile RPMs in a mock chroot
sudo mock -r $MOCK_CONFIG --no-bootstrap-chroot \
    --resultdir $REPO_DIR --with=tests \
    rpmbuild/SRPMS/*.src.rpm osbuild/rpmbuild/SRPMS/*.src.rpm
sudo chown -R $USER ${REPO_DIR}

# Move the logs out of the way.
mv ${REPO_DIR}/*.log $WORKSPACE

# Create a repo of the built RPMs.
createrepo_c ${REPO_DIR}

# Prepare to upload to swift.
mkdir -p ~/.config/openstack
cp $OPENSTACK_CREDS ~/.config/openstack/clouds.yml
export OS_CLOUD=psi

# Upload repository to swift.
pushd repo
    find * -type f -print | xargs openstack object create -f value $CONTAINER
popd