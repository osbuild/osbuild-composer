#!/usr/bin/bash

#
# Test the available distributions. Only allow releases for the current distro.
#
APISOCKET=/run/weldr/api.socket

source /etc/os-release
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Build a grep pattern that results in an empty string when the expected distros are installed
case $ID in
    fedora)
        PATTERN="\[|\]|fedora-"
        ;;
    rhel)
        MAJOR=$(echo "$VERSION_ID" | sed -E 's/\..*//')
        case $MAJOR in
            8)
                # RHEL 8 only supports building RHEL 8
                PATTERN="\[|\]|rhel-8"
                ;;
            9)
                # RHEL 9 supports building RHEL 8 and 9
                PATTERN="\[|\]|rhel-(8|9)"
                ;;
            *)
                # RHEL 10 and later support building all releases
                PATTERN="\[|\]|rhel-.*"
                ;;
        esac
        ;;
    centos)
        MAJOR=$(echo "$VERSION_ID" | sed -E 's/\..*//')
        case $MAJOR in
            8)
                # CentOS 8 only supports building CentosOS 8
                PATTERN="\[|\]|centos-8"
                ;;
            9)
                # CentOS 9 supports building CentosOS 8 and 9
                PATTERN="\[|\]|centos-(8|9)"
                ;;
            *)
                # CentOS 10 and later support building all releases
                PATTERN="\[|\]|centos-.*"
                ;;
        esac
        ;;
    *)
        echo "Unknown distribution id: $ID 😢"
        exit 1
    ;;
esac


# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none
echo "====> Finished Provisioning system"
echo "====> Starting $(basename "$0")"

# Remove repo overrides installed by provision.sh, these will show up in the
# list and cause it to fail and are not needed since this test doesn't build
# anything.
sudo rm -f /etc/osbuild-composer/repositories/*
sudo systemctl try-restart osbuild-composer

echo "Repository directories:"
ls -lR /etc/osbuild-composer/repositories/
ls -lR /usr/share/osbuild-composer/repositories/

echo "Repositories installed by the rpm:"
rpm -qil osbuild-composer-core

# composer-cli in RHEL 8 doesn't support distro command, so use curl for this test
if [ ! -e $APISOCKET ]; then
    echo "osbuild-composer.socket has not been started. 😢"
    exit 1
fi

if ! sudo curl -s --unix-socket $APISOCKET http:///localhost/api/status > /dev/null; then
    echo "osbuild-composer server not available. 😢"
    exit 1
fi

if ! RECOGNIZED_DISTROS=$(sudo curl -s --unix-socket $APISOCKET http:///localhost/api/v1/distros/list | jq -r '.distros[]'); then
    echo "osbuild-composer server error getting distros list. 😢"
    exit 1
fi

# Get a list of all installed distros and compare it with a pattern matching host distribution
# Filter out beta and centos-stream, see GH issue #2257
INSTALLED_DISTROS=$(find "/usr/share/osbuild-composer/repositories" -name '*.json' -printf '%P\n' | awk -F "." '{ print $1 }' | grep -Ev 'beta|stream' | sort)
INSTALLED_REMAINDER=$(echo "$INSTALLED_DISTROS" | grep -v -E "$PATTERN")
# Check if there are any extra distros that match the host pattern but are not recognized
UNRECOGNIZED_DISTROS=$(echo "${INSTALLED_DISTROS}" | grep -v "${RECOGNIZED_DISTROS}")
if [ -n "$INSTALLED_REMAINDER" ] || [ -n "$UNRECOGNIZED_DISTROS" ];then
    echo "Unexpected distros detected:"
    echo "$INSTALLED_REMAINDER"
    echo "$UNRECOGNIZED_DISTROS"
    exit 1
else
    echo "All installed distros are recognized by composer."
fi

# set path to all composer repositories
if [ ! -d "repositories/" ]; then
    git clone --depth 1 http://github.com/osbuild/osbuild-composer
    REPO_PATH="osbuild-composer/repositories/"
else
    REPO_PATH="repositories/"
fi

# ALL_DISTROS - all possible distros from upstream repository
# ALL_EXPECTED_DISTROS - all distros matching host pattern
# ALL_REMAINDERS - all the unrecognized distros
# Filter out beta and centos-stream, see GH issue #2257
ALL_DISTROS=$(find "$REPO_PATH" -name '*.json' -printf '%P\n' | grep -v 'no-aux-key' | awk -F "." '{ print $1 }')
ALL_EXPECTED_DISTROS=$(echo "$ALL_DISTROS" | grep -E "$PATTERN" | grep -Ev 'beta|stream' | sort)
# Warning: filter out the remaining distros by matching whole words to avoid matching
# the value rhel-9X by the pattern rhel-9!
# If we're running on a RHEL 9.X osbuild-composer doesn't know anything about 9.X+1
# images so the value rhel-9.X+1 should be treated as unrecognized and error out as
# expected in the test snippet further below
ALL_REMAINDERS=$(echo "$ALL_DISTROS" | grep -vw "$RECOGNIZED_DISTROS")

echo "DEBUG: ===== ALL_DISTROS ===="
echo "$ALL_DISTROS"
echo "DEBUG: ===== ALL_EXPECTED_DISTROS ===="
echo "$ALL_EXPECTED_DISTROS"
echo "DEBUG: ===== ALL_REMAINDERS ===="
echo "$ALL_REMAINDERS"
echo "DEBUG: ===== END ===="

# Check for any missing distros based on the expected host pattern
if [ "$ALL_EXPECTED_DISTROS" != "$INSTALLED_DISTROS" ];then
    echo "Some distros are missing!"
    echo "Missing distros:"
    diff <(echo "${ALL_EXPECTED_DISTROS}") <(echo "${INSTALLED_DISTROS}") | grep "<" | sed 's/^<\ //g'

    # the check above compares repositories/*.json files from git checkout
    # vs the files installed from an RPM package in order to find files which are
    # not included in the RPM. Don't fail when running on nightly CI pipeline b/c
    # very often the repository will be newer than the downstream RPM.
    if [[ "${CI_PIPELINE_SOURCE:-}" != "schedule" ]]; then
        exit 1
    fi
fi

echo "INFO: Start interating over ALL_REMAINDERS"
# Push a blueprint with unsupported distro to see if composer fails gracefuly
for REMAINING_DISTRO in $ALL_REMAINDERS; do
    echo "INFO: iterating over $REMAINING_DISTRO"

    TEST_BP=blueprint.toml
    tee "$TEST_BP" > /dev/null << EOF
name = "bash"
description = "A base system with bash"
version = "0.0.1"
distro= "$REMAINING_DISTRO"

[[packages]]
name = "bash"
EOF

    set +e
    RESPONSE=$(sudo composer-cli blueprints push $TEST_BP 2>&1)
    set -e

    echo "DEBUG: $REMAINING_DISTRO, RESPONSE=$RESPONSE"

    # there is a different reponse if legacy composer-cli is used
    if rpm -q --quiet weldr-client; then
        if nvrGreaterOrEqual "osbuild-composer" "97"; then
            EXPECTED_RESPONSE="ERROR: BlueprintsError: '$REMAINING_DISTRO' is not a valid distribution (architecture '$(uname -m)')"
        else
            EXPECTED_RESPONSE="ERROR: BlueprintsError: '$REMAINING_DISTRO' is not a valid distribution"
        fi
    else
        EXPECTED_RESPONSE="'$REMAINING_DISTRO' is not a valid distribution"
        RESPONSE=${RESPONSE#*: }
    fi

    if [ "$RESPONSE" == "$EXPECTED_RESPONSE" ];then
            echo "Blueprint push with $REMAINING_DISTRO distro failed as expected."
    else
            echo "Something went wrong during blueprint push test."
            echo "RESPONSE=$RESPONSE"
            echo "EXPECTED_RESPONSE=$EXPECTED_RESPONSE"
            exit 1
    fi
done

echo "🎉 All tests passed."
exit 0
