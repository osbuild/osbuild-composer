#!/bin/bash
set -euvo pipefail

# Get OS details.
source /etc/os-release

# Configure dnf.
echo "fastestmirror=1" >> /etc/dnf/dnf.conf
echo "install_weak_deps=0" >> /etc/dnf/dnf.conf

# Install OS packages.
PACKAGES="createrepo_c dnf-plugins-core git make rpm-build"
[[ $ID == 'centos' ]] && PACKAGES+=" epel-release"
dnf -qy upgrade
dnf -qy install $PACKAGES

# Configure git.
git config --global user.name osbuild
git config --global user.email "nobody@osbuild.org"

# Clone repo.
git clone --recursive https://github.com/osbuild/osbuild-composer
pushd osbuild-composer
    git config --add remote.origin.fetch "+refs/pull/*:refs/remotes/origin/pr/*"
    git fetch --all
    git checkout $GITHUB_SHA
    git log --oneline -5
    git status
popd

# Build source RPM files.
make -C osbuild-composer srpm
make -C osbuild-composer/osbuild srpm
dnf -y builddep \
    osbuild-composer/rpmbuild/SRPMS/*.src.rpm \
    osbuild-composer/osbuild/rpmbuild/SRPMS/*.src.rpm

# Build RPMs.
make -C osbuild-composer rpm
make -C osbuild-compsoer/osbuild rpm
find . -name *.rpm