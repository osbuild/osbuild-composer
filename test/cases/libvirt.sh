#!/bin/bash
set -euo pipefail

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Test the images
/usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2

/usr/libexec/osbuild-composer-test/libvirt_test.sh openstack

# RHEL 8.4 and Centos Stream 8 images also supports uefi, check that
if [[ "$DISTRO_CODE" == "rhel_84" || "$DISTRO_CODE" == "rhel_85" || "$DISTRO_CODE" == "centos_8"  || "$DISTRO_CODE" == "rhel_90" ]]; then
  echo "🐄 Booting qcow2 image in UEFI mode on RHEL/Centos Stream"
  /usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2 uefi
fi
