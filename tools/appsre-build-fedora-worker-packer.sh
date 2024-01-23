#!/bin/bash
# AppSRE runs this script to build an ami and share it with an account
set -exv

export SKIP_CREATE_AMI=false
# Use prebuilt rpms for the fedora images
export BUILD_RPMS=false
export SKIP_TAGS="rpmcopy,subscribe"
export PACKER_ONLY_EXCEPT=--only=amazon-ebs.fedora-38-x86_64,amazon-ebs.fedora-38-aarch64

tools/appsre-build-worker-packer.sh
