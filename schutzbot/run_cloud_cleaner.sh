#!/bin/bash
set -euo pipefail

source /etc/os-release
DISTRO_CODE="${DISTRO_CODE:-${ID}_${VERSION_ID//./}}"
BRANCH_NAME="${BRANCH_NAME:-${CI_COMMIT_BRANCH}}"
BUILD_ID="${BUILD_ID:-${CI_BUILD_ID}}"

CLEANER_CMD="env $(cat "${AZURE_CREDS:-/dev/null}") BRANCH_NAME=$BRANCH_NAME BUILD_ID=$BUILD_ID DISTRO_CODE=$DISTRO_CODE /usr/libexec/osbuild-composer-test/cloud-cleaner"

echo "🧹 Running the cloud cleaner"
$CLEANER_CMD
