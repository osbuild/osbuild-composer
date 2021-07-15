#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

# Set os-variant and boot location used by virt-install.
case "${ID}" in
    "fedora")
        if [ "${VERSION_ID}" -eq "33" ];
        then
            # TODO: make this work for all fedora versions once we can drop the override from
            # the Schutzfile. (osbuild CI doesn't build any Fedora except 33)
            /usr/libexec/tests/osbuild-composer/regression-composer-works-behind-satellite.sh
        else
            echo "No regression test cases for this Fedora version"
        fi;;
    "rhel")
        /usr/libexec/tests/osbuild-composer/regression-include-excluded-packages.sh;;
    "centos")
        /usr/libexec/tests/osbuild-composer/regression-include-excluded-packages.sh;;
    *)
        echo "unsupported distro: ${ID}-${VERSION_ID}"
esac
