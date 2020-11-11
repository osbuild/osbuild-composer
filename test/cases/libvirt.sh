#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

greenprint "Provisioning the software under test"
/usr/libexec/osbuild-composer-test/provision.sh

greenprint "Running libvirt integration tests for qcow2"
/usr/libexec/osbuild-composer/test/libvirt-integration-tests.sh qcow2

greenprint "Running libvirt integration tests for openstack"
/usr/libexec/osbuild-composer/test/libvirt-integration-tests.sh openstack

greenprint "Running libvirt integration tests for vhd"
/usr/libexec/osbuild-composer/test/libvirt-integration-tests.sh vhd
