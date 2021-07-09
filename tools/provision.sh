#!/bin/bash
set -euxo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# koji and ansible are not in RHEL repositories. Depending on them in the spec
# file breaks RHEL gating (see OSCI-1541). Therefore, we need to enable epel
# and install koji and ansible here.
if [[ $ID == rhel || $ID == centos ]]; then
    sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo dnf install -y koji ansible
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
AZURE_CLIENT_ID="${AZURE_CLIENT_ID:-}"
AZURE_CLIENT_SECRET="${AZURE_CLIENT_SECRET:-}"
if [[ -n "$AZURE_CLIENT_ID" && -n "$AZURE_CLIENT_SECRET" ]]; then
    set +x
    sudo tee /etc/osbuild-worker/azure-credentials.toml > /dev/null << EOF
client_id =     "$AZURE_CLIENT_ID"
client_secret = "$AZURE_CLIENT_SECRET"
EOF
    sudo tee -a /etc/osbuild-worker/osbuild-worker.toml > /dev/null << EOF

[azure]
credentials = "/etc/osbuild-worker/azure-credentials.toml"
EOF
    set -x
fi

# Copy rpmrepo snapshots for use in weldr tests
REPODIR=/etc/osbuild-composer/repositories
sudo mkdir -p $REPODIR
# Copy all fedora repo overrides
sudo cp -a /usr/share/tests/osbuild-composer/repositories/{fedora,centos}-*.json "$REPODIR"
# Copy RHEL point relese repos
sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-84.json "$REPODIR"
sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-85.json "$REPODIR"

# RHEL nightly repos need to be overridden in rhel-8.json and rhel-8-beta.json
case "${ID}-${VERSION_ID}" in
    "rhel-8.4")
        # Override old rhel-8.json and rhel-8-beta.json because RHEL 8.4 test needs nightly repos
        sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-84.json "$REPODIR/rhel-8.json"
        # If multiple tests are run and call provision.sh the symlink will need to be overridden with -f
        sudo ln -sf /etc/osbuild-composer/repositories/rhel-8.json "$REPODIR/rhel-8-beta.json"
        ;;
    "rhel-8.5")
        # Override old rhel-8.json and rhel-8-beta.json because RHEL 8.5 test needs nightly repos
        sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-85.json "$REPODIR/rhel-8.json"
        # If multiple tests are run and call provision.sh the symlink will need to be overridden with -f
        sudo ln -sf /etc/osbuild-composer/repositories/rhel-8.json "$REPODIR/rhel-8-beta.json"
        ;;
    *) ;;
esac

# overrides for RHEL nightly builds testing
if [ -f "rhel-8.json" ]; then
    sudo cp rhel-8.json "$REPODIR"
fi

if [ -f "rhel-8-beta.json" ]; then
    sudo cp rhel-8-beta.json "$REPODIR"
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
