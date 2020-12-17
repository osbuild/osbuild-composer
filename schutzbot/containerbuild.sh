#!/bin/bash
set -euo pipefail


# Query host information.
echo "Query host"

ARCH=$(uname -m)
COMMIT=$(git rev-parse HEAD)


# Populate our build matrix.
IMG_TAGS=(
        "quay.io/osbuild/osbuild-composer:f32-${COMMIT}"
        "quay.io/osbuild/osbuild-composer:f33-${COMMIT}"
        "quay.io/osbuild/osbuild-composer:ubi8-${COMMIT}"
)
IMG_PATHS=(
        "./containers/osbuild-composer/"
        "./containers/osbuild-composer/"
        "./containers/osbuild-composer/"
)
IMG_FROMS=(
        "docker.io/library/fedora:32"
        "docker.io/library/fedora:33"
        "registry.access.redhat.com/ubi8"
)
IMG_RPMREPOS=(
        "http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild-composer/fedora-32/${ARCH}/${COMMIT}"
        "http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild-composer/fedora-33/${ARCH}/${COMMIT}"
        "http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild-composer/rhel-8.3/${ARCH}/${COMMIT}"
)
IMG_COUNT=${#IMG_TAGS[*]}


# Prepare host system.
echo "Prepare host system"

sudo dnf -y install podman


# Build the entire matrix.
echo "Build containers"

for ((i=0; i<IMG_COUNT; i++))
do
        i_tag=${IMG_TAGS[$i]}
        i_path=${IMG_PATHS[$i]}
        i_from=${IMG_FROMS[$i]}
        i_rpmrepo=${IMG_RPMREPOS[$i]}

        echo
        echo "-- Build #$i -------------------"
        echo "Tag: ${i_tag}"
        echo "Arch: ${ARCH}"
        echo "Path: ${i_path}"
        echo "From: ${i_from}"
        echo "RpmRepo: ${i_rpmrepo}"
        echo "--------------------------------"
        echo

        podman \
                build \
                "--build-arg=OSB_FROM=${i_from}" \
                "--build-arg=OSB_RPMREPO=${i_rpmrepo}" \
                "--tag=${i_tag}" \
                "${i_path}"
        echo
done
