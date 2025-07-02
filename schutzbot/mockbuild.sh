#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

# function to override template respositores with system repositories which contain rpmrepos snapshots
function template_override {
    sudo dnf -y install jq

    # TODO: remove this, once mock-core-configs ships a template for RHEL-10
    # Use RHEL-9 template as the baseline for now.
    if [[ "$ID" == rhel && ${VERSION_ID%.*} == 10 ]]; then
        TEMPLATE=${ID}-${VERSION_ID%.*}.tpl
        sudo cp /etc/mock/templates/rhel-9.tpl /etc/mock/templates/"$TEMPLATE"
        # change releasever to 10
        sudo sed -i "s/config_opts\['releasever'\] = '9'/config_opts\['releasever'\] = '10'/" /etc/mock/templates/"$TEMPLATE"
        # disable bootstrap image for el10, as there is none yet
        sudo sed -i "s/config_opts\['bootstrap_image_ready'\] = True/config_opts\['bootstrap_image_ready'\] = False/" /etc/mock/templates/"$TEMPLATE"
        # update hardcoded rhel9 paths to rhel10 in repository URLs
        sudo sed -i "s/rhel9/rhel10/g" /etc/mock/templates/"$TEMPLATE"

        sudo cp /etc/mock/rhel-{9,10}-"$(uname -m)".cfg
        sudo sed -i "s/rhel-9/rhel-10/" "/etc/mock/rhel-10-$(uname -m).cfg"
    fi

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

upload_logs() {
    ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
    greenprint "Uploading logs from mock build"
    for path in "${REPO_DIR}"/*.log; do
        file=$(basename -- "$path")
        mv "$path" "${ARTIFACTS}/rpmbuild_${file}"
    done

    ls "${ARTIFACTS}"
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

# EL8 aarch64 builds run out of memory and get killed. A swapfile fixes this.
if [[ "$PLATFORM_ID" == "platform:el8" ]] && [[ "${ARCH}" == "aarch64" ]]; then
    sudo dd if=/dev/zero of=/swapfile bs=1M count=1024
    sudo chmod 600 /swapfile
    sudo mkswap /swapfile
    sudo swapon /swapfile
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
# TODO: Adjust this condition, once EPEL-10 is enabled
if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} -lt 10 ]] && ! rpm -q epel-release; then
    greenprint "📦 Setting up EPEL repository"
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-"${VERSION_ID%.*}".noarch.rpm
    sudo dnf install -y /tmp/epel.rpm
fi

# TODO: Remove this workaround, once EPEL-10 is enabled
if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} == 10 ]]; then
    sudo dnf copr enable -y @osbuild/centpkg "centos-stream-10-$(uname -m)"
fi

# TODO: Remove this workaround, once https://issues.redhat.com/browse/RHEL-49567 is fixed
# We can't workaround this in mock config due to https://github.com/rpm-software-management/mock/pull/1410
if [[ $ID == centos && ${VERSION_ID%.*} == 10 ]]; then
    sudo setenforce 0
    sudo systemctl restart systemd-machined.service
    sudo setenforce 1
fi

# Install requirements for building RPMs in mock.
greenprint "📦 Installing mock requirements"
sudo dnf -y install createrepo_c mock s3cmd podman


# Print some data.
greenprint "🧬 Using mock config: ${MOCK_CONFIG}"
greenprint "📦 SHA: ${COMMIT}"
greenprint "📤 RPMS will be uploaded to: ${REPO_URL}"

# override template repositories
template_override

greenprint "✏ Adding user to mock group"
sudo usermod -a -G mock "$(whoami)"

greenprint "🔧 Building source RPM"
git archive --prefix "osbuild-composer-${COMMIT}/" --output "osbuild-composer-${COMMIT}.tar.gz" HEAD

trap 'upload_logs' ERR

./tools/rpm_spec_add_provides_bundle.sh
mock -r "$MOCK_CONFIG" --buildsrpm \
  --define "commit ${COMMIT}" \
  --spec ./osbuild-composer.spec \
  --config-opts=cleanup_on_failure=False \
  --config-opts=cleanup_on_success=True \
  --sources "./osbuild-composer-${COMMIT}.tar.gz" \
  --resultdir ./srpm

if [ "${NIGHTLY:=false}" == "true" ]; then
    RELAX_REQUIRES="--with=relax_requires"
fi

greenprint "🎁 Building RPMs"
mock -r "$MOCK_CONFIG" \
    --define "commit ${COMMIT}" \
    --config-opts=cleanup_on_failure=False \
    --config-opts=cleanup_on_success=True \
    --with=tests \
    ${RELAX_REQUIRES:+"$RELAX_REQUIRES"} \
    --resultdir "$REPO_DIR" \
    srpm/*.src.rpm

# Change the ownership of all of our repo files from root to our CI user.
sudo chown -R "$USER" "${REPO_DIR%%/*}"

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
