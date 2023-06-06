#!/bin/bash
set -euo pipefail

#
# Helper script that executes `tools/libvirt_test.sh` with the appropiate image type and boot type
#

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Test the images
/usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2

/usr/libexec/osbuild-composer-test/libvirt_test.sh openstack

/usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2 uefi
