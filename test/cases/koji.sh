#!/bin/bash
set -euo pipefail

OSBUILD_COMPOSER_TEST_DATA=/usr/share/tests/osbuild-composer/

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

greenprint "Starting containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh start

greenprint "Adding kerberos config"
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/osbuild-composer/client.keytab
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/osbuild-worker/client.keytab
sudo cp \
    "${OSBUILD_COMPOSER_TEST_DATA}"/kerberos/krb5-local.conf \
    /etc/krb5.conf.d/local

greenprint "Adding the testsuite's CA cert to the system trust store"
sudo cp \
    /etc/osbuild-composer/ca-crt.pem \
    /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
sudo update-ca-trust

greenprint "Restarting composer to pick up new config"
sudo systemctl restart osbuild-composer
sudo systemctl restart osbuild-worker\@1

greenprint "Testing Koji"
koji --server=http://localhost:8080/kojihub --user=osbuild --password=osbuildpass --authtype=password hello

greenprint "Creating Koji task"
koji --server=http://localhost:8080/kojihub --user kojiadmin --password kojipass --authtype=password make-task image

greenprint "Pushing compose to Koji"
sudo /usr/libexec/osbuild-composer-test/koji-compose.py "$DISTRO_CODE" "${ARCH}"

greenprint "Show Koji task"
koji --server=http://localhost:8080/kojihub taskinfo 1
koji --server=http://localhost:8080/kojihub buildinfo 1

greenprint "Run the integration test"
sudo /usr/libexec/osbuild-composer-test/osbuild-koji-tests

greenprint "Stopping containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh stop

greenprint "Removing generated CA cert"
sudo rm \
    /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
sudo update-ca-trust
