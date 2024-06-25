#!/usr/bin/env bash

# just a warning - as "sudo" is not in the container we are using
echo "This script should be run as root"

apt-get update

# This is needed to lint internal/upload/koji package
apt-get install -y libkrb5-dev

# This is needed for the container upload dependencies
apt-get install -y libgpgme-dev

# This is needed for the 'github.com/containers/storage' package
apt-get install -y libbtrfs-dev

apt-get install -y libdevmapper-dev
