#!/bin/bash
# AppSRE runs this script to build an ami and share it with an account
set -exv

export SKIP_CREATE_AMI=false
# Use prebuilt rpms for the fedora images
export BUILD_RPMS=false
# Fedora community workers use osbuild form rpmrepo + composer from
# copr, as the osbuild rpms from copr disappear too quickly.
export SKIP_TAGS="rpmrepo_composer,rpmcopy,subscribe"
FEDORA=fedora-38
export PACKER_ONLY_EXCEPT=--only=amazon-ebs."$FEDORA"-x86_64,amazon-ebs."$FEDORA"-aarch64

# wait up to 30 minutes for the packit-as-a-service app (app id being 29076)
for RETRY in {1..31}; do
    if [ "$RETRY" = 11 ]; then
        echo Waiting for the packit-as-a-service suite failed after 10 minutes
        exit 1
    fi

    CHECK_SUITES=$(curl --request GET --url "https://api.github.com/repos/osbuild/osbuild-composer/commits/$COMMIT_SHA/check-suites?app_id=29076")
    CHECK_SUITES_COUNT=$(echo "$CHECK_SUITES" | jq -r .total_count)
    if [ "$CHECK_SUITES_COUNT" != 1 ]; then
        echo Waiting for the packit-as-a-service suite upstream
        sleep 60
        continue
    fi

    echo Packit-as-a-service suite present \(total_count "$CHECK_SUITES_COUNT"\)
    break
done

CHECK_RUNS_URL=$(echo "$CHECK_SUITES" | jq -r .check_suites[0].check_runs_url)
if [ "$CHECK_RUNS_URL" = null ]; then
    echo CHECK_RUNS_URL not found in "$CHECK_SUITES"
    exit 1
fi

# wait up to 30 minutes for the rpms to be built
for RETRY in {1..31}; do
    if [ "$RETRY" = 31 ]; then
        echo Waiting for the packit-as-a-service results failed after 30 minutes
        exit 1
    fi

    CHECK_RUNS_RESULT=$(curl "$CHECK_RUNS_URL")
    CHECK_RUNS_RESULT_COUNT=$(echo "$CHECK_RUNS_RESULT" | jq -r .total_count)
    if [ "$CHECK_RUNS_RESULT_COUNT" = 0 ] ||  [ "$CHECK_RUNS_RESULT_COUNT" = null ]; then
        echo No results in "$CHECK_RUNS_RESULT", waiting
        sleep 60
        continue
    fi

    X86_RPMS_STATUS=$(echo "$CHECK_RUNS_RESULT" | jq -r ".check_runs[] | select (.name | contains(\"$FEDORA-x86_64\"))" | jq -r .status)
    if [ "$X86_RPMS_STATUS" != completed ]; then
        echo waiting on "$FEDORA"-x86_64 rpms, status is "$X86_RPMS_STATUS"
        sleep 60
        continue
    fi
    AARCH64_RPMS_STATUS=$(echo "$CHECK_RUNS_RESULT" | jq -r ".check_runs[] | select (.name | contains(\"$FEDORA-aarch64\"))" | jq -r .status)
    if [ "$AARCH64_RPMS_STATUS" != completed ]; then
        echo waiting on "$FEDORA"-aarch64 rpms, status is "$AARCH64_RPMS_STATUS"
        sleep 60
        continue
    fi

    X86_RPMS_CONCLUSION=$(echo "$CHECK_RUNS_RESULT" | jq -r ".check_runs[] | select (.name | contains(\"$FEDORA-x86_64\"))" | jq -r .conclusion)
    if [ "$X86_RPMS_CONCLUSION" = success ]; then
        echo "$FEDORA"-x86_64 rpms ready!
    else
        echo "$FEDORA"-x86_64 rpms failed to build :\(
        exit 1
    fi
    AARCH64_RPMS_CONCLUSION=$(echo "$CHECK_RUNS_RESULT" | jq -r ".check_runs[] | select (.name | contains(\"$FEDORA-aarch64\"))" | jq -r .conclusion)
    if [ "$AARCH64_RPMS_CONCLUSION" = success ]; then
        echo "$FEDORA"-aarch64 rpms ready!
        break
    else
        echo "$FEDORA"-aarch64 rpms failed to build :\(
        exit 1
    fi
done

tools/appsre-build-worker-packer.sh
