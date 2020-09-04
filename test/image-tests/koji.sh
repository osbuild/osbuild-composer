#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

if [[ $ID == rhel ]] && ! rpm -q epel-release; then
    greenprint "📦 Setting up EPEL repository"
    curl -Ls --retry 5 --output /tmp/epel.rpm \
        https://dl.fedoraproject.org/pub/epel/epel-release-latest-8.noarch.rpm
    sudo rpm -Uvh /tmp/epel.rpm
fi

greenprint "Installing required packages"
sudo dnf -y install \
    container-selinux \
    dnsmasq \
    krb5-workstation \
    koji \
    podman \
    python3 \
    sssd-krb5

if [[ $ID == rhel ]]; then
  greenprint "Tweaking podman, maybe."
  sudo cp schutzbot/vendor/87-podman-bridge.conflist /etc/cni/net.d/
  sudo cp schutzbot/vendor/dnsname /usr/libexec/cni/
fi

greenprint "Starting containers"
sudo ./internal/upload/koji/run-koji-container.sh start

greenprint "Adding kerberos config"
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/krb5.keytab
sudo cp \
    test/image-tests/krb5-local.conf \
    /etc/krb5.conf.d/local

greenprint "Initializing Kerberos"
kinit osbuild-krb@LOCAL -k
sudo -u _osbuild-composer kinit osbuild-krb@LOCAL -k

greenprint "Adding generated CA cert for Koji"
sudo cp \
    /tmp/osbuild-composer-koji-test/ca-crt.pem \
    /etc/pki/ca-trust/source/anchors/koji-ca-crt.pem
sudo update-ca-trust

greenprint "Restarting composer to pick up new certs"
sudo systemctl restart osbuild-composer

greenprint "Testing Koji"
koji --server=http://localhost/kojihub --user=osbuild --password=osbuildpass --authtype=password hello
koji --server=http://localhost/kojihub hello
sudo -u _osbuild-composer koji --server=http://localhost/kojihub hello

greenprint "Creating Koji task"
koji --server=http://localhost/kojihub --user kojiadmin --password kojipass --authtype=password make-task image

greenprint "Pushing compose to Koji"
sudo ./test/image-tests/koji-compose.py "${ID}-${VERSION_ID%.*}"

greenprint "Show Koji task"
koji --server=http://localhost/kojihub taskinfo 1
koji --server=http://localhost/kojihub buildinfo 1

greenprint "Stopping containers"
sudo ./internal/upload/koji/run-koji-container.sh stop

greenprint "Removing generated CA cert"
sudo rm \
    /etc/pki/ca-trust/source/anchors/koji-ca-crt.pem
sudo update-ca-trust
