#!/usr/bin/bash

#
# The objective of this script is to provide special requirement needed
# for generate-test-cases. For example, disable the dnf-json daemon
# and then execute generate-test-cases.
#

set -euxo pipefail

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Get OS data
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Colorful timestamped output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

# install requirements
sudo dnf -y install go

# stop dnf-json socket
sudo systemctl stop osbuild-dnf-json.socket

# test the test cases generation
WORKDIR=$(mktemp -d -p /var/tmp)

OSBUILD_LABEL=$(matchpathcon -n "$(which osbuild)")
chcon "$OSBUILD_LABEL" tools/image-info

# test the test case generation when dnf-json socket is stopped
sudo ./tools/test-case-generators/generate-test-cases\
    --output test/data/manifests\
    --arch x86_64\
    --distro rhel-8\
    --image-type qcow2\
    --store "$WORKDIR"\
    --keep-image-info
