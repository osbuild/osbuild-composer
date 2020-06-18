#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m${1}\033[0m"
}

# Get OS details.
source /etc/os-release

# Install EPEL for RHEL so we can get mock.
if [[ $ID == rhel ]]; then
    greenprint "📦 Installing EPEL for RHEL"
    sudo curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
    sudo dnf makecache
fi

# Install packages.
greenprint "📦 Installing packages for mock build"
sudo dnf -qy install createrepo_c make mock rpm-build

# Install s3cmd if it is not present.
greenprint "📦 Installing s3cmd"
sudo pip3 -qq install s3cmd

# Jenkins sets a workspace variable as the root of its working directory.
WORKSPACE=${WORKSPACE:-$(pwd)}

# Mock configuration file to use for building RPMs.
MOCK_CONFIG="${ID}-${VERSION_ID%.*}-$(uname -m)"

# Jenkins takes the proposed PR and merges it onto master. Although this
# creates a new SHA (which is slightly confusing), it ensures that the code
# merges properly against master and it tests the code against the latest
# commit in master, which is certainly good.
POST_MERGE_SHA=$(git rev-parse --short HEAD)

# Bucket in S3 where our artifacts are uploaded
REPO_BUCKET=osbuild-composer-repos

# Public URL for the S3 bucket with our artifacts.
MOCK_REPO_BASE_URL="http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com"

# Directory to hold the RPMs temporarily before we upload them.
REPO_DIR=repo/${JOB_NAME}/${POST_MERGE_SHA}/${ID}${VERSION_ID//./}

# Full URL to the RPM repository after they are uploaded.
REPO_URL=${MOCK_REPO_BASE_URL}/${JOB_NAME}/${POST_MERGE_SHA}/${ID}${VERSION_ID//./}

# Print some data.
greenprint "🧬 Using mock config: ${MOCK_CONFIG}"
greenprint "📦 Post merge SHA: ${POST_MERGE_SHA}"
greenprint "📤 RPMS will be uploaded to: ${REPO_URL}"

# Build source RPMs.
greenprint "🔧 Building source RPMs."
make srpm
make -C osbuild srpm

# Fix RHEL 8 mock template for non-subscribed images.
if [[ $NODE_NAME == *rhel8[23]* ]]; then
    greenprint "📋 Updating RHEL 8 mock template for unsubscribed image"
    sudo curl --retry 5 -Lsko /etc/mock/templates/rhel-8.tpl \
        https://gitlab.cee.redhat.com/snippets/2208/raw
fi

# Compile RPMs in a mock chroot
greenprint "🎁 Building RPMs with mock"
sudo mock -r $MOCK_CONFIG --no-bootstrap-chroot \
    --resultdir $REPO_DIR --with=tests \
    rpmbuild/SRPMS/*.src.rpm osbuild/rpmbuild/SRPMS/*.src.rpm
sudo chown -R $USER ${REPO_DIR}

# Move the logs out of the way.
greenprint "🧹 Retaining logs from mock build"
mv ${REPO_DIR}/*.log $WORKSPACE

# Create a repo of the built RPMs.
greenprint "⛓️ Creating dnf repository"
createrepo_c ${REPO_DIR}

# Upload repository to S3.
greenprint "☁ Uploading RPMs to S3"
pushd repo
    s3cmd --acl-public sync . s3://${REPO_BUCKET}/
popd

# Create a repository file.
greenprint "📜 Generating dnf repository file"
tee osbuild-mock.repo << EOF
[osbuild-mock]
name=osbuild mock ${JOB_NAME}-${POST_MERGE_SHA} ${ID}${VERSION_ID//./}
baseurl=${REPO_URL}
enabled=1
gpgcheck=0
# Default dnf repo priority is 99. Lower number means higher priority.
priority=5
EOF