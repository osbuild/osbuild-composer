#!/bin/bash
set -euo pipefail

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Test the images
/usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2

#TODO: remove this condition once there is rhel9 support for openstack and vhd image types
if [[ "$DISTRO_CODE" != rhel-90 ]]; then
  /usr/libexec/osbuild-composer-test/libvirt_test.sh openstack
fi

# RHEL 8.4 and Centos Stream 8 images also supports uefi, check that
if [[ "$DISTRO_CODE" == "rhel-84" || "$DISTRO_CODE" == "centos-8" ]]; then
  echo "🐄 Booting qcow2 image in UEFI mode on RHEL/Centos Stream"
  /usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2 uefi
fi
