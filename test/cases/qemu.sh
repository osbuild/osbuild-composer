#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

greenprint "Running qemu integration tests for qcow2"
/usr/libexec/osbuild-composer/test/qemu-integration-tests.sh qcow2

greenprint "Running qemu integration tests for openstack"
/usr/libexec/osbuild-composer/test/qemu-integration-tests.sh openstack

greenprint "Running qemu integration tests for vhd"
/usr/libexec/osbuild-composer/test/qemu-integration-tests.sh vhd
