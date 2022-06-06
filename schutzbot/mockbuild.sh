#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

# function to override template respositores with system repositories which contain rpmrepos snapshots
function template_override {
    sudo dnf -y install jq
    if sudo subscription-manager status; then
        greenprint "📋 Running on subscribed RHEL machine, no mock template override done."
        return 0
    fi
    if [[ "$ID" == rhel ]]; then
        TEMPLATE=${ID}-${VERSION_ID%.*}.tpl
        # disable subscription for nightlies
        sudo sed -i "s/config_opts\['redhat_subscription_required'\] = True/config_opts['redhat_subscription_required'] = False/" /etc/mock/templates/"$TEMPLATE"
    elif [[ "$ID" == fedora ]]; then
        TEMPLATE=fedora-branched.tpl
    elif [[ "$ID" == centos ]]; then
        TEMPLATE=${ID}-stream-${VERSION_ID}.tpl
        STREAM=-stream
    fi
    greenprint "📋 Updating $ID-$VERSION_ID mock template with rpmrepo snapshot repositories"
    REPOS=$(jq -r ."\"${ID}${STREAM:-}-${VERSION_ID}\".repos[].file" Schutzfile)
    sudo sed -i '/user_agent/q' /etc/mock/templates/"$TEMPLATE"
    for REPO in $REPOS; do
        sudo cat "$REPO" | sudo tee -a /etc/mock/templates/"$TEMPLATE"
    done
    echo '"""' | sudo tee -a /etc/mock/templates/"$TEMPLATE"
}

# Get OS and architecture details.
source tools/set-env-variables.sh

# Mock configuration file to use for building RPMs.
MOCK_CONFIG="${ID}-${VERSION_ID%.*}-$(uname -m)"

if [[ $ID == centos ]]; then
  MOCK_CONFIG="centos-stream-${VERSION_ID%.*}-$(uname -m)"
fi

# The commit this script operates on.
COMMIT=$(git rev-parse HEAD)

# Bucket in S3 where our artifacts are uploaded
REPO_BUCKET=osbuild-composer-repos

# Public URL for the S3 bucket with our artifacts.
MOCK_REPO_BASE_URL="http://${REPO_BUCKET}.s3-website.us-east-2.amazonaws.com"

# Distro version in whose buildroot was the RPM built.
DISTRO_VERSION=${ID}-${VERSION_ID}

if [[ "$ID" == rhel ]] && sudo subscription-manager status; then
  # If this script runs on a subscribed RHEL, the RPMs are actually built
  # using the latest CDN content, therefore rhel-*-cdn is used as the distro
  # version.
  DISTRO_VERSION=rhel-${VERSION_ID%.*}-cdn
fi

# Relative path of the repository – used for constructing both the local and
# remote paths below, so that they're consistent.
REPO_PATH=osbuild-composer/${DISTRO_VERSION}/${ARCH}/${COMMIT}
if [ "${NIGHTLY:=false}" == "true" ]; then
    REPO_PATH=nightly/${REPO_PATH}
fi

# Directory to hold the RPMs temporarily before we upload them.
REPO_DIR=repo/${REPO_PATH}

# Full URL to the RPM repository after they are uploaded.
REPO_URL=${MOCK_REPO_BASE_URL}/${REPO_PATH}

# Don't rerun the build if it already exists
if curl --silent --fail --head --output /dev/null "${REPO_URL}/repodata/repomd.xml"; then
  greenprint "🎁 Repository already exists. Exiting."
  exit 0
fi

# Mock and s3cmd is only available in EPEL for RHEL.
if [[ $ID == rhel || $ID == centos ]] && ! rpm -q epel-release; then
    greenprint "📦 Setting up EPEL repository"
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-"${VERSION_ID%.*}".noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
fi

# Install requirements for building RPMs in mock.
greenprint "📦 Installing mock requirements"
sudo dnf -y install createrepo_c mock s3cmd


# Print some data.
greenprint "🧬 Using mock config: ${MOCK_CONFIG}"
greenprint "📦 SHA: ${COMMIT}"
greenprint "📤 RPMS will be uploaded to: ${REPO_URL}"

# override template repositories
template_override

greenprint "🔧 Building source RPM"
git archive --prefix "osbuild-composer-${COMMIT}/" --output "osbuild-composer-${COMMIT}.tar.gz" HEAD
./tools/rpm_spec_add_provides_bundle.sh
sudo mock -r "$MOCK_CONFIG" --buildsrpm \
  --define "commit ${COMMIT}" \
  --spec ./osbuild-composer.spec \
  --sources "./osbuild-composer-${COMMIT}.tar.gz" \
  --resultdir ./srpm

if [ "${NIGHTLY:=false}" == "true" ]; then
    RELAX_REQUIRES="--with=relax_requires"
fi

greenprint "🎁 Building RPMs"
sudo mock -r "$MOCK_CONFIG" \
    --define "commit ${COMMIT}" \
    --with=tests \
    ${RELAX_REQUIRES:+"$RELAX_REQUIRES"} \
    --resultdir "$REPO_DIR" \
    srpm/*.src.rpm

# Change the ownership of all of our repo files from root to our CI user.
sudo chown -R "$USER" "${REPO_DIR%%/*}"

greenprint "🧹 Remove logs from mock build"
rm "${REPO_DIR}"/*.log

# leave only -tests RPM to minimize interference when installing
# osbuild-composer.rpm from distro repositories
if [ "${NIGHTLY:=false}" == "true" ]; then
    find "${REPO_DIR}" -type f -not -name "osbuild-composer-tests*.rpm" -exec rm -f "{}" \;
fi

# Create a repo of the built RPMs.
greenprint "⛓️ Creating dnf repository"
createrepo_c "${REPO_DIR}"

# Upload repository to S3.
greenprint "☁ Uploading RPMs to S3"
pushd repo
    AWS_ACCESS_KEY_ID="$V2_AWS_ACCESS_KEY_ID" \
    AWS_SECRET_ACCESS_KEY="$V2_AWS_SECRET_ACCESS_KEY" \
    s3cmd --acl-public put --recursive . s3://${REPO_BUCKET}/
popd
