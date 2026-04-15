#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Verify container source type in compose manifests JSON.
# Args: $1 = manifests JSON, $2 = expected source ("remote" or "local")
function verifyContainerSourceType() {
    local MANIFESTS="$1"
    local EXPECTED="${2:-remote}"

    local HAS_SKOPEO HAS_LOCAL
    HAS_SKOPEO=$(echo "$MANIFESTS" | jq -r '.manifests[0].sources | has("org.osbuild.skopeo")')
    HAS_LOCAL=$(echo "$MANIFESTS" | jq -r '.manifests[0].sources | has("org.osbuild.containers-storage")')
    echo "has org.osbuild.skopeo: $HAS_SKOPEO"
    echo "has org.osbuild.containers-storage: $HAS_LOCAL"

    case "$EXPECTED" in
        "remote")
            test "$HAS_SKOPEO" = "true"
            test "$HAS_LOCAL" = "false"
            ;;
        "local")
            test "$HAS_SKOPEO" = "false"
            test "$HAS_LOCAL" = "true"
            ;;
        *)
            redprint "Invalid expected container source type: $EXPECTED, expected 'remote' or 'local'"
            exit 1
            ;;
    esac
}
