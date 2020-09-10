#!/bin/bash
set -euo pipefail

CLEANER_CMD="env $(cat "$AZURE_CREDS") CHANGE_ID=$CHANGE_ID BUILD_ID=$BUILD_ID DISTRO_CODE=$DISTRO_CODE /usr/libexec/osbuild-composer/cloud-cleaner"

echo "🧹 Running the cloud cleaner"
$CLEANER_CMD
