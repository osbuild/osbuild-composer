#!/bin/bash
set -euxo pipefail

source /etc/os-release

# koji and ansible are not in RHEL repositories. Depending on them in the spec
# file breaks RHEL gating (see OSCI-1541). Therefore, we need to enable epel
# and install koji and ansible here.
if [[ $ID == rhel || $ID == centos ]]; then
    sudo dnf install -y https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo dnf install -y koji ansible
fi

sudo mkdir -p /etc/osbuild-composer
sudo cp -a /usr/share/tests/osbuild-composer/composer/*.toml \
    /etc/osbuild-composer/

# Copy rpmrepo snapshots for use in weldr tests
sudo mkdir -p /etc/osbuild-composer/repositories
# This script is now RHEL-8.4 specific, install the only override:
sudo cp /usr/share/tests/osbuild-composer/repositories/rhel-84.json /etc/osbuild-composer/repositories/rhel-8.json
# If multiple tests are run and call provision.sh the symlink will need to be overriden with -f
sudo ln -sf /etc/osbuild-composer/repositories/rhel-8.json /etc/osbuild-composer/repositories/rhel-8-beta.json

# overrides for RHEL nightly builds testing
if [ -f "rhel-8.json" ]; then
    sudo mv rhel-8.json /etc/osbuild-composer/repositories/
fi

if [ -f "rhel-8-beta.json" ]; then
    sudo mv rhel-8-beta.json /etc/osbuild-composer/repositories/
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
