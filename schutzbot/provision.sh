#!/bin/bash
set -euxo pipefail

sudo mkdir -p /etc/osbuild-composer
sudo cp -a /usr/share/tests/osbuild-composer/composer/*.toml \
    /etc/osbuild-composer/

sudo cp -a /usr/share/tests/osbuild-composer/ca/* \
    /etc/osbuild-composer/
sudo chown _osbuild-composer /etc/osbuild-composer/composer-*.pem

sudo systemctl start osbuild-remote-worker.socket
sudo systemctl start osbuild-composer.socket

if rpm -q osbuild-composer-koji; then
    sudo systemctl start osbuild-composer-koji.socket
fi

if rpm -q osbuild-composer-cloud; then
    sudo systemctl start osbuild-composer-cloud.socket
fi

# Basic verification
sudo composer-cli status show
sudo composer-cli sources list
for SOURCE in $(sudo composer-cli sources list); do
    sudo composer-cli sources info "$SOURCE"
done
