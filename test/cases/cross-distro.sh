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
            9)
                # CentOS 9 supports building CentosOS 9
                PATTERN="\[|\]|centos-(9)"
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
sudo rm -rf /etc/osbuild-composer/repositories
sudo systemctl stop 'osbuild*.service'
sudo composer-cli status show

echo "Repository directories:"
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
INSTALLED_DISTROS=$(find "/usr/share/osbuild-composer/repositories" -name '*.json' -printf '%P\n' | sed 's/\.[^.]*$//' | grep -Ev 'beta|stream' | sort)
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

# determine the 'osbuild/images' repository version used by the osbuild-composer
sudo dnf install -y golang
COMPOSER_DEPS=$(go version -m /usr/libexec/osbuild-composer/osbuild-composer)
IMAGES_VERSION=$(echo "$COMPOSER_DEPS" | sed -n 's|^\t\+dep\t\+github\.com/osbuild/images\t\+\(v[0-9.a-zA-Z-]\+\)\t\+$|\1|p')
if [ -z "$IMAGES_VERSION" ]; then
    echo "ERROR: Unable to determine osbuild/images version from osbuild-composer binary. Composer deps:"
    echo "$COMPOSER_DEPS"
    exit 1
fi

greenprint "INFO: Using osbuild/images version to check repo configs: $IMAGES_VERSION"
git clone http://github.com/osbuild/images
( cd images && git checkout "$IMAGES_VERSION" )
REPO_PATH="images/data/repositories/"

# ALL_DISTROS - all possible distros from upstream repository
# ALL_EXPECTED_DISTROS - all distros matching host pattern
# ALL_REMAINDERS - all the unrecognized distros
# Filter out beta and centos-stream, see GH issue #2257
ALL_DISTROS=$(find "$REPO_PATH" -name '*.json' -printf '%P\n' | grep -v 'no-aux-key' | sed 's/\.[^.]*$//')
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
echo "DEBUG: ===== INSTALLED_DISTROS ===="
echo "$INSTALLED_DISTROS"
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
        EXPECTED_RESPONSE="ERROR: BlueprintsError: '$REMAINING_DISTRO' is not a valid distribution (architecture '$(uname -m)')"
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

# Function to start a compose
# TODO: This function should be moved to shared_lib.sh
function start_compose() {
    local blueprint=$1
    local image_type=${2:-qcow2}

    local compose_start
    compose_start=$(mktemp)
    local compose_id

    greenprint "🚀 Starting compose of $image_type for $blueprint blueprint"
    sudo composer-cli --json compose start "$blueprint" "$image_type" | tee "$compose_start" >&2
    compose_id=$(get_build_info ".build_id" "$compose_start")

    greenprint "INFO: Compose started with ID: ${compose_id}"
    echo "$compose_id"
}

# Function to wait for a compose to finish
# TODO: This function should be moved to shared_lib.sh
function wait_for_compose() {
    local compose_id=$1
    local timeout=${2:-600}
    local compose_status

    if [[ -z "$compose_id" ]]; then
        redprint "ERROR (wait_for_compose): No compose ID provided"
        exit 1
    fi

    local compose_info
    compose_info=$(mktemp)

    greenprint "⏱ Waiting for compose to finish: ${compose_id}"
    while [[ $timeout -gt 0 ]]; do
        sudo composer-cli --json compose info "${compose_id}" | tee "$compose_info" > /dev/null
        compose_status=$(get_build_info ".queue_status" "$compose_info")

        # Is the compose finished?
        if [[ $compose_status != "RUNNING" ]] && [[ $compose_status != "WAITING" ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 30
        timeout=$((timeout - 30))
    done

    # Get the last compose status if the compose was still running before the last sleep
    if [[ $compose_status == "RUNNING" ]]; then
        sudo composer-cli --json compose info "${compose_id}" | tee "$compose_info" > /dev/null
        compose_status=$(get_build_info ".queue_status" "$compose_info")
    fi

    if [[ $compose_status == "RUNNING" || $compose_status == "WAITING" ]] && [[ timeout -le 0 ]]; then
        redprint "ERROR: Compose did not finish in time"
        exit 1
    fi

    greenprint "INFO: Compose finished with status: ${compose_status}"

    # Return the status of the compose
    echo "$compose_status"
}

# Get the compose log.
# TODO: This function should be moved to shared_lib.sh
function get_compose_log() {
    local compose_id=$1

    if [[ -z "$compose_id" ]]; then
        redprint "ERROR (get_compose_log): No compose ID provided"
        exit 1
    fi

    sudo composer-cli compose log "$compose_id"
}

# Function to ensure the system is subscribed
# Subscription is need to build RHEL GA images
function ensure_subscription() {
    if sudo subscription-manager status; then
        greenprint "📋 Running on subscribed RHEL machine"
    elif [[ -f "$V2_RHN_REGISTRATION_SCRIPT" ]]; then
        greenprint "📋 Registering the system using registration script"
        sudo bash "$V2_RHN_REGISTRATION_SCRIPT"
        # Since the system was not registered, it didn't depend on the CDN repos, so don't enable them
        sudo subscription-manager config --rhsm.manage_repos=1
    else
        redprint "ERROR: Not running on a subscribed RHEL machine and no registration script provided"
        exit 1
    fi
}

# Function to build a vanilla image for a given distro
function test_cross_build_distro() {
    local distro=$1
    # default to gce image type, because building it will try importing all GPG keys that we ship in repo configs
    local image_type=${2:-gce}

    if [[ -z "$distro" ]]; then
        redprint "ERROR (cross_build_distro): No distro provided"
        exit 1
    fi

    greenprint "Testing cross-distro build of $distro ($image_type)"
    local blueprint
    blueprint=$(mktemp --suffix=".toml")

    local bp_name="cross-distro-$distro"
    cat > "$blueprint" << EOF
name = "$bp_name"
distro = "$distro"
EOF

    echo "INFO: $blueprint content:"
    cat "$blueprint"

    sudo composer-cli blueprints push "$blueprint"
    local compose_id
    compose_id=$(start_compose "$bp_name" "$image_type")
    local compose_status
    compose_status=$(wait_for_compose "$compose_id")
    
    if [[ $compose_status != "FINISHED" ]]; then
        redprint "ERROR: Compose did not finish successfully ($compose_status)"
        redprint "INFO: Compose logs for $compose_id:"
        get_compose_log "$compose_id"
        exit 1
    fi
}

# Test cross-distro builds on RHEL and CentOS
case $ID in
    rhel)
        MAJOR=$(echo "$VERSION_ID" | sed -E 's/\..*//')
        ensure_subscription
        case $MAJOR in
            9)
                if ! nvrGreaterOrEqual "osbuild-composer" "132.1"; then
                    yellowprint "WARNING: osbuild-composer version lower than 132.1 is known to have issues with el8 on el9 cross-distro builds. Skipping test."
                    exit 0
                fi
                # There are no new RHEL-8 releases, so just use the distro alias
                test_cross_build_distro "rhel-8"
                ;;
            10)
                # There are no new RHEL-8 releases, so just use the distro alias
                test_cross_build_distro "rhel-8"
                # Test building RHEL 9.5, which is the latest RHEL-9 minor version that is GA at this time
                test_cross_build_distro "rhel-9.5"
                ;;
            *)
                greenprint "INFO not testing actual cross-distro image build on $ID-$VERSION_ID"
                ;;
        esac
        ;;
    centos)
        MAJOR=$(echo "$VERSION_ID" | sed -E 's/\..*//')
        case $MAJOR in
            10)
                test_cross_build_distro "centos-9"
                ;;
            *)
                greenprint "INFO not testing actual cross-distro image build on $ID-$VERSION_ID"
                ;;
        esac
        ;;
    *)
        greenprint "INFO not testing actual cross-distro image build on $ID-$VERSION_ID"
        ;;
esac


echo "🎉 All tests passed."
exit 0
