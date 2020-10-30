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
# The whole generation is done in a $CADIR to better represent how osbuild-ca
# it.
CERTDIR=/etc/osbuild-composer
OPENSSL_CONFIG=/usr/share/tests/osbuild-composer/x509/openssl.cnf
CADIR=/etc/osbuild-composer-test/ca

# The $CADIR might exist from a previous test (current Schutzbot's imperfection)
sudo rm -rf $CADIR || true
sudo mkdir -p $CADIR

pushd $CADIR
    sudo mkdir certs private
    sudo touch index.txt

    # Generate a CA.
    sudo openssl req -config $OPENSSL_CONFIG \
        -keyout private/ca.key.pem \
        -new -nodes -x509 -extensions osbuild_ca_ext \
        -out ca.cert.pem -subj "/CN=osbuild.org"

    # Copy the private key to the location expected by the tests
    sudo cp ca.cert.pem "$CERTDIR"/ca-crt.pem

    # Generate a composer certificate.
    sudo openssl req -config $OPENSSL_CONFIG \
        -keyout "$CERTDIR"/composer-key.pem \
        -new -nodes \
        -out /tmp/composer-csr.pem \
        -subj "/CN=localhost/emailAddress=osbuild@example.com" \
        -addext "subjectAltName=DNS:localhost"

    sudo openssl ca -batch -config $OPENSSL_CONFIG \
        -extensions osbuild_server_ext \
        -in /tmp/composer-csr.pem \
        -out "$CERTDIR"/composer-crt.pem

    sudo chown _osbuild-composer "$CERTDIR"/composer-*.pem

    # Generate a worker certificate.
    sudo openssl req -config $OPENSSL_CONFIG \
        -keyout "$CERTDIR"/worker-key.pem \
        -new -nodes \
        -out /tmp/worker-csr.pem \
        -subj "/CN=localhost/emailAddress=osbuild@example.com" \
        -addext "subjectAltName=DNS:localhost"

    sudo openssl ca -batch -config $OPENSSL_CONFIG \
        -extensions osbuild_client_ext \
        -in /tmp/worker-csr.pem \
        -out "$CERTDIR"/worker-crt.pem

    # Generate a client certificate.
    sudo openssl req -config $OPENSSL_CONFIG \
        -keyout "$CERTDIR"/client-key.pem \
        -new -nodes \
        -out /tmp/client-csr.pem \
        -subj "/CN=client.osbuild.org/emailAddress=osbuild@example.com" \
        -addext "subjectAltName=DNS:client.osbuild.org"

    sudo openssl ca -batch -config $OPENSSL_CONFIG \
        -extensions osbuild_client_ext \
        -in /tmp/client-csr.pem \
        -out "$CERTDIR"/client-crt.pem

    # Client keys are used by tests to access the composer APIs. Allow all users access.
    sudo chmod 644 "$CERTDIR"/client-key.pem

popd

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
