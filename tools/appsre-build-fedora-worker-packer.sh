#!/bin/bash
# AppSRE runs this script to build an ami and share it with an account
set -exv

export SKIP_CREATE_AMI=false
# Use prebuilt rpms for the fedora images
export BUILD_RPMS=false
export SKIP_TAGS="rpmcopy,subscribe"
FEDORA=fedora-38
export PACKER_ONLY_EXCEPT=--only=amazon-ebs."$FEDORA"-x86_64,amazon-ebs."$FEDORA"-aarch64

# wait until the rpms are built upstream
COMMIT_SHA="${COMMIT_SHA:-$(git rev-parse HEAD)}"
while true; do
    RET=$(curl -w "%{http_code}" -s -o /dev/null http://osbuild-composer-repos.s3.amazonaws.com/osbuild-composer/"$FEDORA"/x86_64/"$COMMIT_SHA"/state.log)
    if [ "$RET" != 200 ]; then
        sleep 30
        continue
    fi
    RET=$(curl -w "%{http_code}" -s -o /dev/null http://osbuild-composer-repos.s3.amazonaws.com/osbuild-composer/"$FEDORA"/aarch64/"$COMMIT_SHA"/state.log)
    if [ "$RET" != 200 ]; then
        sleep 30
        continue
    fi
    break
done

tools/appsre-build-worker-packer.sh
