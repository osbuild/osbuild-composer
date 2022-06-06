#!/usr/bin/bash

# Save the list of bundled packages into a file
WORKDIR=$(mktemp -d)
BUNDLES_FILE=${WORKDIR}/bundles.txt
awk '{print "Provides: bundled(golang("$1")) = "$2}' go.mod | sort --ignore-case | uniq | sed -e 's/-/_/g' -e '/bundled(golang())/d' -e '/bundled(golang(go\|module\|replace\|require))/d' > "$BUNDLES_FILE"

# Remove the current bundle lines
sed -i '/^# BUNDLE_START/,/^# BUNDLE_END/{//p;d;}' osbuild-composer.spec
# Add the new bundle lines
sed -i "/^# BUNDLE_START/r ${BUNDLES_FILE}" osbuild-composer.spec
