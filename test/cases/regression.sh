#!/bin/bash
set -euo pipefail

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

# Set os-variant and boot location used by virt-install.
case "${ID}" in
    "fedora")
        echo "No regression test for Fedora";;
    "rhel")
        /usr/libexec/tests/osbuild-composer/regression-include-excluded-packages.sh;;
    "centos")
        /usr/libexec/tests/osbuild-composer/regression-include-excluded-packages.sh;;
    *)
        echo "unsupported distro: ${ID}-${VERSION_ID}"
esac
