#!/bin/bash
set -euo pipefail

#
# Helper script that executes `tools/libvirt_test.sh` with the appropiate image type and boot type
#

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Test the images
/usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2

/usr/libexec/osbuild-composer-test/libvirt_test.sh openstack

# RHEL 8.4+ and Centos Stream 8+ images also supports uefi, check that
if [[ "$DISTRO_CODE" == "rhel-84" || "$DISTRO_CODE" == "rhel-85" || "$DISTRO_CODE" == "centos-8" || "$DISTRO_CODE" == "rhel-90" || "$DISTRO_CODE" == "centos-9" ]]; then
  echo "🐄 Booting qcow2 image in UEFI mode on RHEL/Centos Stream"
  /usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2 uefi
fi
