#!/bin/bash
set -euxo pipefail

sudo mkdir -p /etc/osbuild-composer
sudo cp -a /usr/share/tests/osbuild-composer/composer/*.toml \
    /etc/osbuild-composer/


# Copy Fedora rpmrepo snapshots for use in weldr tests. RHEL's are usually more
# stable, and not available publically from rpmrepo.
sudo mkdir -p /etc/osbuild-composer/repositories
sudo cp -a /usr/share/tests/osbuild-composer/repositories/fedora-*.json \
    /etc/osbuild-composer/repositories/

# Generate all X.509 certificates for the tests
CERTDIR=/etc/osbuild-composer

# Generate a CA.
sudo /usr/libexec/osbuild-composer-test/x509/generate-certificate \
    -selfsigned \
    -out "$CERTDIR"/ca-crt.pem \
    -keyout "$CERTDIR"/ca-key.pem \
    -cn ca.osbuild.org \
    -san ca.osbuild.org

# Generate a composer certificate.
sudo /usr/libexec/osbuild-composer-test/x509/generate-certificate \
    -CA "$CERTDIR"/ca-crt.pem \
    -CAkey "$CERTDIR"/ca-key.pem \
    -out "$CERTDIR"/composer-crt.pem \
    -keyout "$CERTDIR"/composer-key.pem \
    -cn localhost \
    -san localhost
sudo chown _osbuild-composer "$CERTDIR"/composer-*.pem

# Generate a worker certificate.
sudo /usr/libexec/osbuild-composer-test/x509/generate-certificate \
    -CA "$CERTDIR"/ca-crt.pem \
    -CAkey "$CERTDIR"/ca-key.pem \
    -out "$CERTDIR"/worker-crt.pem \
    -keyout "$CERTDIR"/worker-key.pem \
    -cn localhost \
    -san localhost

# Generate a client certificate.
sudo /usr/libexec/osbuild-composer-test/x509/generate-certificate \
    -CA "$CERTDIR"/ca-crt.pem \
    -CAkey "$CERTDIR"/ca-key.pem \
    -out "$CERTDIR"/client-crt.pem \
    -keyout "$CERTDIR"/client-key.pem \
    -cn client.osbuild.org \
    -san client.osbuild.org

# Client keys are used by tests to access the composer APIs. Allow all users access.
sudo chmod 644 "$CERTDIR"/client-key.pem

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
