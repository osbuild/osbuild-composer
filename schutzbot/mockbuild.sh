#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m${1}\033[0m"
}

# Get OS and architecture details.
source /etc/os-release
ARCH=$(uname -m)

# Mock is only available in EPEL for RHEL.
if [[ $ID == rhel ]] && ! rpm -q epel-release; then
    greenprint "📦 Setting up EPEL repository"
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
fi

# Register RHEL if we are provided with a registration script.
if [[ -n "${RHN_REGISTRATION_SCRIPT:-}" ]] && ! sudo subscription-manager status; then
    greenprint "🪙 Registering RHEL instance"
    sudo chmod +x $RHN_REGISTRATION_SCRIPT
    sudo $RHN_REGISTRATION_SCRIPT
fi

# Install requirements for building RPMs in mock.
greenprint "📦 Installing mock requirements"
sudo dnf -y install createrepo_c make mock python3-pip rpm-build

# Install s3cmd if it is not present.
if ! s3cmd --version > /dev/null 2>&1; then
    greenprint "📦 Installing s3cmd"
    sudo pip3 -q install s3cmd
fi

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
REPO_DIR=repo/${JOB_NAME}/${POST_MERGE_SHA}/${ID}${VERSION_ID//./}_${ARCH}

# Full URL to the RPM repository after they are uploaded.
REPO_URL=${MOCK_REPO_BASE_URL}/${JOB_NAME}/${POST_MERGE_SHA}/${ID}${VERSION_ID//./}_${ARCH}

# Print some data.
greenprint "🧬 Using mock config: ${MOCK_CONFIG}"
greenprint "📦 Post merge SHA: ${POST_MERGE_SHA}"
greenprint "📤 RPMS will be uploaded to: ${REPO_URL}"

# Build source RPMs.
greenprint "🔧 Building source RPMs."
make srpm
make -C osbuild srpm

# Fix RHEL 8 mock template for non-subscribed images.
if [[ "${ID}${VERSION_ID//./}" == rhel83 ]]; then
    greenprint "📋 Updating RHEL 8 mock template for unsubscribed image"
    sudo cp $NIGHTLY_MOCK_TEMPLATE /etc/mock/templates/rhel-8.tpl
    cat $NIGHTLY_REPO | sudo tee -a /etc/mock/templates/rhel-8.tpl > /dev/null
    echo '"""' | sudo tee -a /etc/mock/templates/rhel-8.tpl > /dev/null
fi

# Compile RPMs in a mock chroot
greenprint "🎁 Building RPMs with mock"
sudo mock -v -r $MOCK_CONFIG --resultdir $REPO_DIR --with=tests \
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