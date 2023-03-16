#!/usr/bin/bash

#
# Test the ability to specify custom repositories
#
set -euo pipefail

source /etc/os-release

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Set up variables.
case "${ID}-${VERSION_ID}" in
    fedora*)
        ;;
    rhel-*)
        ;;
    centos-*)
        ;;
    *)
        echo "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac

TEST_UUID=$(uuidgen)
IMAGE_KEY="osbuild-composer-test-${TEST_UUID}"
ARTIFACTS="ci-artifacts"
mkdir -p "${ARTIFACTS}"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# Workaround the problem that 'image-info' can not read SELinux labels unknown to the host from the image
OSBUILD_LABEL=$(matchpathcon -n "$(which osbuild)")
sudo chcon "$OSBUILD_LABEL" /usr/libexec/osbuild-composer-test/image-info

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "${COMPOSE_ID}" | tee "${LOG_FILE}" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.json

    # Download the metadata.
    sudo composer-cli compose metadata "${COMPOSE_ID}" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "${TARBALL}" -C "${TEMPDIR}"
    sudo rm -f "${TARBALL}"

    # Move the JSON file into place.
    sudo cat "${TEMPDIR}"/"${COMPOSE_ID}".json | jq -M '.' | tee "${METADATA_FILE}" > /dev/null
}

# Build ostree image.
build_image() {
    blueprint_name=$1
    image_type=$2

    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!
    # Stop watching the worker journal when exiting.
    trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

    # Start the compose.
    greenprint "🚀 Starting compose"
    sudo composer-cli --json compose start "${blueprint_name}" "${image_type}" | tee "${COMPOSE_START}"
    COMPOSE_ID=$(get_build_info ".build_id" "${COMPOSE_START}")

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "${COMPOSE_INFO}" > /dev/null
        COMPOSE_STATUS=$(get_build_info ".queue_status" "${COMPOSE_INFO}")

        # Is the compose finished?
        if [[ ${COMPOSE_STATUS} != RUNNING ]] && [[ ${COMPOSE_STATUS} != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 5
    done

    # Capture the compose logs from osbuild.
    greenprint "💬 Getting compose log and metadata"
    get_compose_log "${COMPOSE_ID}"
    get_compose_metadata "${COMPOSE_ID}"

    # Kill the journal monitor immediately and remove the trap
    sudo pkill -P "${WORKER_JOURNAL_PID}"
    trap - EXIT

    # Did the compose finish with success?
    if [[ ${COMPOSE_STATUS} != FINISHED ]]; then
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi
}

greenprint "🚀 Checking custom repositories"

REPO_ID="example"
REPO_NAME="Example repo"
REPO_FILENAME="custom.repo"
REPO_BASEURL="https://example.com/download/yum"
REPO_GPGKEY_URL="https://example.com/example-key.asc"
REPO_GPGKEY="-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQGiBGRBSJURBACzCoe9UNfxOUiFLq9b60weSBFdr39mLViscecDWATNvXtgRoK/\nxl/4qpayzALRCQ2Ek/pMrbKPF/3ngECuBv7S+rI4n/rIia4FNcqzYeZAz4DE4NP/\neUGvz49tWhmH17hX/rmF9kz5kLq2bDZI4GDgZW/oMDdt2ivj092Ljm9jRwCgyQy3\nWEK6RJvIcSEh9vbdwVdMPOcD/iHqNejTMFwGyZfCWB0eIOoxUOUn/ZZpELTL2UpW\nGduCf3txb5SkK7M+WDbb0S5IvNXoi0tc13STiD6Oxg2O9PkSvvYb+8zxlhNoSTwy\n54j7Rf5FlnQ3TAFfjtQ5LCx56LKK73j4RjvKW//ktm5n54exsgo9Ry/e12T46dRg\n7tIlA/91rzLm57Qyc73A7zjgIzef9O6V5ZzowC+pp/jfb5pS9hXgROekLkMgX0vg\niA5rM5OpqK4bArVP1lRWnLyvghwO+TW763RVuXlS0scfzMy4g0NgrG6j7TIOKEqz\n4xQxOuwkudqiQr/kOqKuLxQBXa+5MJkyhfPmqYw5wpqyCwFa/7Q4b3NidWlsZCB0\nZXN0IChvc2J1aWxkIHRlc3QgZ3Bna2V5KSA8b3NidWlsZEBleGFtcGxlLmNvbT6I\newQTEQIAOxYhBGB8woiEPRKBO8Cr31lulpQgMejzBQJkQUiVAhsjBQsJCAcCAiIC\nBhUKCQgLAgQWAgMBAh4HAheAAAoJEFlulpQgMejzapMAoLmUg1mNDTRUaCrN/fzm\nHYLHL6jkAJ9pEKkJQiHB6SfD0fkiD2GkELYLubkBDQRkQUiVEAQAlAAXrQ572vuw\nxI3W8GSZmOQiAYOQmOKRloLEy6VZ3NSOb9y2TXj33QTkJBPOM17AzB7E+YjZrpUt\ngl6LlXmfjMcJAcXhFaUBCilAcMwMlLl7DtnSkLnLIXYmHiN0v83BH/H0EPutOc5l\n0QIyugutifp9SJz2+EWpC4bjA7GFkQ8AAwUD/1tLEGqCJ37O8gfzYt2PWkqBEoOY\n0Z3zwVS6PWW/IIkak9dAJ0iX5NMeFWpzFNfviDPHqhEdUR55zsxyUZIZlCX5jwmA\nt7qm3cbH4HNU1Ogq3Q9hykbTPWPZVkpvNm/TO8TA2brhkz3nuS8Hbmh+rjXFOSZj\nDQBUxItuuj2hhpQEiGAEGBECACAWIQRgfMKIhD0SgTvAq99ZbpaUIDHo8wUCZEFI\nlQIbDAAKCRBZbpaUIDHo83fQAKDHgFIaggaNsvDQkj7vMX0fecHRhACfS9Bvxn2W\nWSb6T+gChmYBseZwk/k=\n=DQ3i\n-----END PGP PUBLIC KEY BLOCK-----\n"
REPO_GPGCHECK="true"
REPO_ENABLED="true"

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "custom-repo"
description = "A base system with custom repositories enabled"
version = "0.0.1"

[[customizations.repositories]]
id="${REPO_ID}"
name="${REPO_NAME}"
filename="${REPO_FILENAME}"
baseurls=[ "${REPO_BASEURL}" ]
gpgkeys=[ "${REPO_GPGKEY}", "${REPO_GPGKEY_URL}" ]
gpgcheck=${REPO_GPGCHECK}
enabled=${REPO_ENABLED}
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing custom-repo blueprint"
sudo composer-cli blueprints push "${BLUEPRINT_FILE}"
sudo composer-cli blueprints depsolve custom-repo

build_image custom-repo qcow2

# Download the image
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "💬 Getting image info"
INFO="$(sudo /usr/libexec/osbuild-composer-test/image-info "${IMAGE_FILENAME}")"

# Clean compose and blueprints.
greenprint "🧼 Clean up osbuild-composer"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete custom-repo > /dev/null

greenprint "📗 Checking results"
CUSTOM_REPO_EXISTS=$(jq --arg r "custom.repo" 'any(.yum_repos[] | keys | .[] == $r; .)' <<< "${INFO}")
echo "CUSTOM_REPO_EXISTS: ${CUSTOM_REPO_EXISTS}"
if "${CUSTOM_REPO_EXISTS}"; then
    greenprint "✅ Custom image-builder repo file has been created"
else
    echo "❌ Custom repo has not been created"
    exit 1
fi

REPO_CONTAINS_PATH_TO_KEY=$(jq --arg r "$REPO_ID" --arg k "file:///etc/pki/rpm-gpg/RPM-GPG-KEY-$REPO_ID-0" \
  '.yum_repos[]."custom.repo" | to_entries | map(select(.key == $r) | .value) | any(.[] | .gpgkey | contains($k); .)' \
  <<< "${INFO}")
echo "REPO_CONTAINS_PATH_TO_KEY ${REPO_CONTAINS_PATH_TO_KEY}"
if "${REPO_CONTAINS_PATH_TO_KEY}"; then
    greenprint "✅ Custom image-builder repo file contains gpgkey file location"
else
    echo "❌ Custom repo does not contain gpgkey file location"
    exit 1
fi

REPO_CONTAINS_KEY_URL=$(jq --arg r "$REPO_ID" --arg k "file:///etc/pki/rpm-gpg/RPM-GPG-KEY-$REPO_ID-0" \
  '.yum_repos[]."custom.repo" | to_entries | map(select(.key == $r) | .value) | any(.[] | .gpgkey | contains($k); .)' \
  <<< "${INFO}")
echo "REPO_CONTAINS_KEY_URL: ${REPO_CONTAINS_KEY_URL}"
if "${REPO_CONTAINS_KEY_URL}"; then
    greenprint "✅ Custom image-builder repo file contains gpgkey url"
else
    echo "❌ Custom repo does not contain gpgkey url"
    exit 1
fi

echo "🎉 All tests passed."
exit 0
