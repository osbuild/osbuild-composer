#!/bin/bash
set -euo pipefail

CLEANER_CMD="env $(cat "${AZURE_CREDS:-/dev/null}") BRANCH_NAME=$BRANCH_NAME BUILD_ID=$BUILD_ID DISTRO_CODE=$DISTRO_CODE /usr/libexec/osbuild-composer-test/cloud-cleaner"

echo "🧹 Running the cloud cleaner"
$CLEANER_CMD
