#!/usr/bin/bash

SPEC_FILE=${1:-"osbuild-composer.spec"}

# Save the list of bundled packages into a file
WORKDIR=$(mktemp -d)
BUNDLES_FILE=${WORKDIR}/bundles.txt
grep "^# " vendor/modules.txt | awk '{print "Provides: bundled(golang("$2")) = "$3}' | sort --ignore-case | uniq | sed -e 's/-/_/g' > "${BUNDLES_FILE}"

# Remove the current bundle lines
sed -i '/^# BUNDLE_START/,/^# BUNDLE_END/{//p;d;}' "${SPEC_FILE}"
# Add the new bundle lines
sed -i "/^# BUNDLE_START/r ${BUNDLES_FILE}" "${SPEC_FILE}"
