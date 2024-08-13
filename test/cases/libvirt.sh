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

# Fedora's openstack image is an alias of qcow2, we don't need to test it separately
# el10 / c10s no longer has an openstack image type
if [[ ("$ID" == "rhel" || "$ID" == "centos") && ${VERSION_ID%.*} -lt 10 ]] ; then
    /usr/libexec/osbuild-composer-test/libvirt_test.sh openstack
fi

/usr/libexec/osbuild-composer-test/libvirt_test.sh qcow2 uefi
