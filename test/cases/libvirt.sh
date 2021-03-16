#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Test the images (bios is supported only on x86_64
if [[ "$ARCH" == "x86_64" ]]; then
  /usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2
  /usr/libexec/osbuild-composer-test/libvirt_test.sh openstack
  /usr/libexec/osbuild-composer-test/libvirt_test.sh vhd
fi

# RHEL 8.4, Centos Stream 8 and all aarch64 images also supports uefi, check that
if [[ "${ID}-${VERSION_ID}" == "rhel-8.4" || "${ID}-${VERSION_ID}" == "centos-8" || "$ARCH" == "aarch64" ]]; then
  echo "🐄 Booting qcow2 image in UEFI mode"
  /usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2 uefi
  /usr/libexec/osbuild-composer-test/libvirt_test.sh openstack uefi
fi
