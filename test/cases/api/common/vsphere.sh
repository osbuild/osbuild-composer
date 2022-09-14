#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

function installClientVSphere() {
    if ! hash govc; then
        ARCH="$(uname -m)"
        if [ "$ARCH" = "aarch64" ]; then
            ARCH="arm64"
        fi
        greenprint "Installing govc"
        pushd "${WORKDIR}" || exit 1
        curl -Ls --retry 5 --output govc.tar.gz \
            "https://github.com/vmware/govmomi/releases/download/v0.29.0/govc_Linux_$ARCH.tar.gz"
        tar -xvf govc.tar.gz
        GOVC_CMD="${WORKDIR}/govc"
        chmod +x "${GOVC_CMD}"
        popd || exit 1
    else
        echo "Using pre-installed 'govc' from the system"
        GOVC_CMD="govc"
    fi

    $GOVC_CMD version
}

