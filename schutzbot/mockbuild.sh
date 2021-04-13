#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m${1}\033[0m"
}

# Get OS and architecture details.
source /etc/os-release
ARCH=$(uname -m)

# Mock configuration file to use for building RPMs.
MOCK_CONFIG="${ID}-${VERSION_ID%.*}-$(uname -m)"

if [[ $ID == centos ]]; then
  MOCK_CONFIG="centos-stream-8-$(uname -m)"
fi

# The commit this script operates on.
COMMIT=$(git rev-parse HEAD)

# Bucket in S3 where our artifacts are uploaded
REPO_BUCKET=osbuild-composer-repos

# Public URL for the S3 bucket with our artifacts.
MOCK_REPO_BASE_URL="http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com"

# Used in the gitlab CI proof of concept so it can upload its rpms to
# a different location.
EXTRA_REPO_PATH_SEGMENT="${EXTRA_REPO_PATH_SEGMENT:-}"

# Relative path of the repository – used for constructing both the local and
# remote paths below, so that they're consistent.
REPO_PATH=${EXTRA_REPO_PATH_SEGMENT}osbuild-composer/${ID}-${VERSION_ID}/${ARCH}/${COMMIT}

# Directory to hold the RPMs temporarily before we upload them.
REPO_DIR=repo/${REPO_PATH}

# Full URL to the RPM repository after they are uploaded.
REPO_URL=${MOCK_REPO_BASE_URL}/${REPO_PATH}

# Don't rerun the build if it already exists
if curl --silent --fail --head --output /dev/null "${REPO_URL}/repodata/repomd.xml"; then
  greenprint "🎁 Repository already exists. Exiting."
  exit 0
fi

# TODO: update to install correct version once epel-9 is out
# Mock and s3cmd is only available in EPEL for RHEL.
if [[ $ID == rhel || $ID == centos ]] && ! rpm -q epel-release; then
    greenprint "📦 Setting up EPEL repository"
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
fi

# Register RHEL if we are provided with a registration script.
if [[ -n "${RHN_REGISTRATION_SCRIPT:-}" ]] && ! sudo subscription-manager status; then
    greenprint "🪙 Registering RHEL instance"
    sudo chmod +x "$RHN_REGISTRATION_SCRIPT"
    sudo "$RHN_REGISTRATION_SCRIPT"
fi

# Install requirements for building RPMs in mock.
greenprint "📦 Installing mock requirements"
sudo dnf -y install createrepo_c mock s3cmd


# Print some data.
greenprint "🧬 Using mock config: ${MOCK_CONFIG}"
greenprint "📦 SHA: ${COMMIT}"
greenprint "📤 RPMS will be uploaded to: ${REPO_URL}"

# rhel 8.4 will run off of the internal repos and does not have a redhat subscription
if [[ $VERSION_ID == 8.4 ]]; then
    greenprint "📋 Updating RHEL 8 mock template for unsubscribed image"
    sudo sed -i '/# repos/q' /etc/mock/templates/rhel-8.tpl
    # remove the subscription check
    sudo sed -i "s/config_opts\['redhat_subscription_required'\] = True/config_opts['redhat_subscription_required'] = False/" /etc/mock/templates/rhel-8.tpl
    cat "$RHEL84_NIGHTLY_REPO" | sudo tee -a /etc/mock/templates/rhel-8.tpl > /dev/null
    # We need triple quotes at the end of the template to mark the end of the repo list.
    echo '"""' | sudo tee -a /etc/mock/templates/rhel-8.tpl
fi

# Use chroot() call instead of nspawn until a bug in F34 is fixed
# https://bugzilla.redhat.com/show_bug.cgi?id=1931452
if [[ "$NAME" == "Fedora" && "$VERSION_ID" == "34" ]]; then
  MOCK_WORKAROUND="--isolation=simple"
fi

greenprint "🔧 Building source RPM"
git archive --prefix "osbuild-composer-${COMMIT}/" --output "osbuild-composer-${COMMIT}.tar.gz" HEAD
sudo mock -r "$MOCK_CONFIG" --buildsrpm \
  --define "commit ${COMMIT}" \
  --spec ./osbuild-composer.spec \
  --sources "./osbuild-composer-${COMMIT}.tar.gz" \
  ${MOCK_WORKAROUND:-} \
  --resultdir ./srpm

greenprint "🎁 Building RPMs"
sudo mock -r "$MOCK_CONFIG" \
    --define "commit ${COMMIT}" \
    --with=tests \
    --resultdir "$REPO_DIR" \
    ${MOCK_WORKAROUND:-} \
    srpm/*.src.rpm

# Change the ownership of all of our repo files from root to our CI user.
sudo chown -R "$USER" "${REPO_DIR%%/*}"

greenprint "🧹 Remove logs from mock build"
rm "${REPO_DIR}"/*.log

# Create a repo of the built RPMs.
greenprint "⛓️ Creating dnf repository"
createrepo_c "${REPO_DIR}"

# Upload repository to S3.
greenprint "☁ Uploading RPMs to S3"
pushd repo
    s3cmd --acl-public sync . s3://${REPO_BUCKET}/
popd
