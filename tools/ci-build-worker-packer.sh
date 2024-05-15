#!/bin/bash
# AppSRE runs this script to build an ami and share it with an account
set -exv

COMMIT_SHA=$(git rev-parse HEAD)
COMMIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
SKIP_CREATE_AMI=false
BUILD_RPMS=false
# Use prebuilt rpms on CI
ANSIBLE_TAGS="ci"

if [ -n "$CI_COMMIT_SHA" ]; then
    COMMIT_SHA="$CI_COMMIT_SHA"
fi

if [ -n "$CI_COMMIT_BRANCH" ]; then
    COMMIT_BRANCH="$CI_COMMIT_BRANCH"
fi

# skip creating AMIs on PRs to save a ton of resources
if [[ $COMMIT_BRANCH == PR-* ]]; then
    SKIP_CREATE_AMI=true
fi

if [ -n "$CI_COMMIT_BRANCH" ] && [ "$CI_COMMIT_BRANCH" == "main" ]; then
    # Schutzbot on main: build all except rhel
    PACKER_ONLY_EXCEPT=--except=amazon-ebs.rhel-9-x86_64,amazon-ebs.rhel-9-aarch64
else
    # Schutzbot but not main, build everything (use dummy except)
    PACKER_ONLY_EXCEPT=--except=amazon-ebs.dummy
fi

export COMMIT_SHA COMMIT_BRANCH SKIP_CREATE_AMI BUILD_RPMS ANSIBLE_TAGS PACKER_ONLY_EXCEPT
tools/appsre-build-worker-packer.sh
