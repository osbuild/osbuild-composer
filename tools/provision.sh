#!/bin/bash
set -euxo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# koji and ansible are not in RHEL repositories. Depending on them in the spec
# file breaks RHEL gating (see OSCI-1541). Therefore, we need to enable epel
# and install koji and ansible here.
if [[ $ID == rhel || $ID == centos ]] && [[ ${VERSION_ID%.*} == 8 ]] && ! rpm -q epel-release; then
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
    sudo dnf install -y koji ansible
elif [[ $ID == rhel || $ID == centos ]] && [[ ${VERSION_ID%.*} == 9 ]]; then
    # we have our own small epel for EL9, let's install it

    # install Red Hat certificate, otherwise dnf copr fails
    curl -LO --insecure https://hdn.corp.redhat.com/rhel8-csb/RPMS/noarch/redhat-internal-cert-install-0.1-23.el7.csb.noarch.rpm
    sudo dnf install -y ./redhat-internal-cert-install-0.1-23.el7.csb.noarch.rpm dnf-plugins-core
    sudo dnf copr enable -y copr.devel.redhat.com/osbuild-team/epel-el9 "rhel-9.dev-$ARCH"
fi

# RHEL 8.6+ and CentOS 9 require different handling for ansible
ge86=$(echo "${VERSION_ID}" | awk '{print $1 >= 8.6}')  # do a numerical comparison for the version
echo -n "${ID}=${VERSION_ID} "
if [[ "${ID}" == "rhel" || "${ID}" == "centos" ]] && (( ge86 )); then
    sudo dnf install -y ansible-core koji

    # NOTE: do we need this?
    sudo ansible-galaxy collection install community.general
fi

sudo mkdir -p /etc/osbuild-composer
sudo cp -a /usr/share/tests/osbuild-composer/composer/osbuild-composer.toml \
    /etc/osbuild-composer/

sudo mkdir -p /etc/osbuild-worker
sudo cp -a /usr/share/tests/osbuild-composer/worker/osbuild-worker.toml \
    /etc/osbuild-worker/

# if GCP credentials are defined in the ENV, add them to the worker's configuration
GOOGLE_APPLICATION_CREDENTIALS="${GOOGLE_APPLICATION_CREDENTIALS:-}"
if [ -n "$GOOGLE_APPLICATION_CREDENTIALS" ]; then
    # The credentials file must be copied to a different location. Jenkins places
    # it into /tmp and as a result, the worker would not see it due to using PrivateTmp=true.
    GCP_CREDS_WORKER_PATH="/etc/osbuild-worker/gcp-credentials.json"
    sudo cp "$GOOGLE_APPLICATION_CREDENTIALS" "$GCP_CREDS_WORKER_PATH"
    echo -e "\n[gcp]\ncredentials = \"$GCP_CREDS_WORKER_PATH\"\n" | sudo tee -a /etc/osbuild-worker/osbuild-worker.toml
fi

# if Azure credentials are defined in the env, create the credentials file
V2_AZURE_CLIENT_ID="${V2_AZURE_CLIENT_ID:-}"
V2_AZURE_CLIENT_SECRET="${V2_AZURE_CLIENT_SECRET:-}"
if [[ -n "$V2_AZURE_CLIENT_ID" && -n "$V2_AZURE_CLIENT_SECRET" ]]; then
    set +x
    sudo tee /etc/osbuild-worker/azure-credentials.toml > /dev/null << EOF
client_id =     "$V2_AZURE_CLIENT_ID"
client_secret = "$V2_AZURE_CLIENT_SECRET"
EOF
    sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[azure]
credentials = "/etc/osbuild-worker/azure-credentials.toml"
EOF
    set -x
fi

# if AWS credentials are defined in the ENV, add them to the worker's configuration
V2_AWS_ACCESS_KEY_ID="${V2_AWS_ACCESS_KEY_ID:-}"
V2_AWS_SECRET_ACCESS_KEY="${V2_AWS_SECRET_ACCESS_KEY:-}"
if [[ -n "$V2_AWS_ACCESS_KEY_ID" && -n "$V2_AWS_SECRET_ACCESS_KEY" ]]; then
    set +x
    sudo tee /etc/osbuild-worker/aws-credentials.toml > /dev/null << EOF
[default]
aws_access_key_id = "$V2_AWS_ACCESS_KEY_ID"
aws_secret_access_key = "$V2_AWS_SECRET_ACCESS_KEY"
EOF
    sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[aws]
credentials = "/etc/osbuild-worker/aws-credentials.toml"
EOF
    set -x
fi

# Copy rpmrepo snapshots for use in weldr tests
REPODIR=/etc/osbuild-composer/repositories
sudo mkdir -p $REPODIR
# Copy all fedora repo overrides
sudo cp -a /usr/share/tests/osbuild-composer/repositories/{fedora,centos}-*.json "$REPODIR"
# Copy RHEL point relese repos
sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-85.json "$REPODIR"
sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-86.json "$REPODIR"
sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-90.json "$REPODIR"

# RHEL nightly repos need to be overridden
case "${ID}-${VERSION_ID}" in
    "rhel-8.6")
        # Override old rhel-8.json and rhel-8-beta.json because RHEL 8.6 test needs nightly repos
        sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-86.json "$REPODIR/rhel-8.json"
        # If multiple tests are run and call provision.sh the symlink will need to be overridden with -f
        sudo ln -sf /etc/osbuild-composer/repositories/rhel-8.json "$REPODIR/rhel-8-beta.json"
        ;;
    "rhel-9.0")
        # Override old rhel-90.json and rhel-90-beta.json because RHEL 9.0 test needs nightly repos
        sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-90.json "$REPODIR/rhel-90.json"
        # If multiple tests are run and call provision.sh the symlink will need to be overridden with -f
        sudo ln -sf /etc/osbuild-composer/repositories/rhel-90.json "$REPODIR/rhel-90-beta.json"
        ;;
    *) ;;
esac

# overrides for RHEL nightly builds testing
VERSION_SUFFIX=$(echo "${VERSION_ID}" | tr -d ".")
if [ -f "rhel-${VERSION_ID%.*}.json" ]; then
    sudo cp rhel-"${VERSION_ID%.*}".json "$REPODIR/rhel-${VERSION_SUFFIX}.json"
fi

if [ -f "rhel-${VERSION_ID%.*}-beta.json" ]; then
    sudo cp rhel-"${VERSION_ID%.*}"-beta.json "$REPODIR/rhel-${VERSION_SUFFIX}-beta.json"
fi

# Generate all X.509 certificates for the tests
# The whole generation is done in a $CADIR to better represent how osbuild-ca
# it.
CERTDIR=/etc/osbuild-composer
OPENSSL_CONFIG=/usr/share/tests/osbuild-composer/x509/openssl.cnf
CADIR=/etc/osbuild-composer-test/ca

scriptloc=$(dirname "$0")
sudo "${scriptloc}/gen-certs.sh" "${OPENSSL_CONFIG}" "${CERTDIR}" "${CADIR}"
sudo chown _osbuild-composer "${CERTDIR}"/composer-*.pem

sudo systemctl start osbuild-remote-worker.socket
sudo systemctl start osbuild-composer.socket
sudo systemctl start osbuild-composer-api.socket

# The keys were regenerated but osbuild-composer might be already running.
# Let's try to restart it. In ideal world, this shouldn't be needed as every
# test case is supposed to run on a pristine machine. However, this is
# currently not true on Schutzbot
sudo systemctl try-restart osbuild-composer

# Basic verification
sudo composer-cli status show
sudo composer-cli sources list
for SOURCE in $(sudo composer-cli sources list); do
    sudo composer-cli sources info "$SOURCE"
done
