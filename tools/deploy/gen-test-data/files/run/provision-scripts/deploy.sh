#!/bin/bash

set -euxo pipefail

dnf -y install  osbuild \
                osbuild-selinux \
                osbuild-ostree \
                osbuild-composer \
                golang
