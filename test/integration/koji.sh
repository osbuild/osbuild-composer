#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release

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

greenprint "Copying custom worker config"
sudo mkdir -p /etc/osbuild-worker
sudo cp test/composer/osbuild-worker.toml \
    /etc/osbuild-worker/

greenprint "Adding kerberos config"
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/osbuild-composer/client.keytab
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/osbuild-worker/client.keytab
sudo cp \
    test/kerberos/krb5-local.conf \
    /etc/krb5.conf.d/local

greenprint "Adding generated CA cert for Koji"
sudo cp \
    /tmp/osbuild-composer-koji-test/ca-crt.pem \
    /etc/pki/ca-trust/source/anchors/koji-ca-crt.pem
sudo update-ca-trust

greenprint "Restarting composer to pick up new config"
sudo systemctl restart osbuild-composer
sudo systemctl restart osbuild-worker\@1

greenprint "Testing Koji"
koji --server=http://localhost:8080/kojihub --user=osbuild --password=osbuildpass --authtype=password hello

greenprint "Creating Koji task"
koji --server=http://localhost:8080/kojihub --user kojiadmin --password kojipass --authtype=password make-task image

greenprint "Pushing compose to Koji"
sudo ./tools/koji-compose.py "${ID}-${VERSION_ID%.*}"

greenprint "Show Koji task"
koji --server=http://localhost:8080/kojihub taskinfo 1
koji --server=http://localhost:8080/kojihub buildinfo 1

greenprint "Stopping containers"
sudo ./internal/upload/koji/run-koji-container.sh stop

greenprint "Removing generated CA cert"
sudo rm \
    /etc/pki/ca-trust/source/anchors/koji-ca-crt.pem
sudo update-ca-trust
