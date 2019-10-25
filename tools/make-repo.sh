#!/bin/bash

set -e

DISTRO=$1
VERSION=$2
ARCH=$3

TEMPDIR=$(mktemp -d -t osbuild-composer-repo-XXXXXXX)

mock -r ${DISTRO}-${VERSION}-${ARCH} --scm-enable --scm-option method=git --scm-option package=golang-github-osbuild-composer --scm-option git_get=set --scm-option spec="golang-github-osbuild-composer.spec" \
      --scm-option branch=HEAD --scm-option write_tar=True --scm-option git_get="git clone --recurse-submodules ${PWD} golang-github-osbuild-composer" --resultdir ${TEMPDIR}
mock -r ${DISTRO}-${VERSION}-${ARCH} --scm-enable --scm-option method=git --scm-option package=osbuild --scm-option git_get=set --scm-option spec="osbuild.spec" \
      --scm-option branch=HEAD --scm-option write_tar=True --scm-option git_get="git clone --recurse-submodules ${PWD}/osbuild" --resultdir ${TEMPDIR}

createrepo_c ${TEMPDIR}

sha256sum ${TEMPDIR}/repodata/repomd.xml

python3 -m http.server --directory ${TEMPDIR}

rm -rf ${TEMPDIR}
