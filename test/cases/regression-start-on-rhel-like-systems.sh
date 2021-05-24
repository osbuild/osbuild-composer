#!/bin/bash
set -xeuo pipefail

function cleanup()
{
    echo "Running cleanup function"
    sudo mv /etc/os-release.backup /etc/os-release || echo "There was no backup for etc/os-release."

}

trap cleanup EXIT

sudo mv /etc/os-release /etc/os-release.backup
sudo tee /etc/os-release << STOPHERE
NAME="Rocky Linux"
VERSION="8"
ID="rocky"
ID_LIKE="rhel fedora"
VERSION_ID="8"
PLATFORM_ID="platform:el8"
PRETTY_NAME="Rocky Linux 8"
ANSI_COLOR="0;31"
CPE_NAME="cpe:/o:rocky:rocky:8"
HOME_URL="https://rockylinux.org/"
BUG_REPORT_URL="https://bugs.rockylinux.org/"
ROCKY_SUPPORT_PRODUCT="Rocky Linux"
ROCKY_SUPPORT_PRODUCT_VERSION="8"
STOPHERE

if ! sudo systemctl restart osbuild-composer;
then
    journalctl -xe --unit osbuild-composer
    exit 1
fi