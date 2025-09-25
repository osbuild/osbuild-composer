#!/usr/bin/bash

#
# Test the ability to specify custom mountpoints
#
set -euo pipefail

source /etc/os-release

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

TEST_UUID=$(uuidgen)
IMAGE_KEY="osbuild-composer-test-${TEST_UUID}"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

CUSTOMIZATION_TYPE="${1:-filesystem}"

function cleanup_on_exit() {
    greenprint "🧼 Cleaning up"
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"

    # since this function can be called at any time, ensure that we don't expand unbound variables
    IMAGE_FILENAME="${IMAGE_FILENAME:-}"
    [ "$IMAGE_FILENAME" ] && sudo rm -f "$IMAGE_FILENAME"

    # Remove tmp dir.
    sudo rm -rf "$TEMPDIR"

}
trap cleanup_on_exit EXIT

build_image() {
    blueprint_file=$1
    blueprint_name=$2
    image_type=$3
    want_fail=$4

    # Prepare the blueprint for the compose.
    greenprint "📋 Preparing blueprint"
    sudo composer-cli blueprints push "$blueprint_file"
    greenprint "📋 Depsolving blueprint"
    sudo composer-cli blueprints depsolve "$blueprint_name"

    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!

    # Start the compose.
    greenprint "🚀 Starting compose"
    # this needs "|| true" at the end for the fail case scenario
    sudo composer-cli --json compose start "$blueprint_name" "$image_type" | tee "$COMPOSE_START" || true
    STATUS=$(get_build_info ".status" "$COMPOSE_START")

    if [[ $want_fail == "$STATUS" ]]; then
        redprint "Something went wrong with the compose. 😢"
        sudo pkill -P ${WORKER_JOURNAL_PID}
        exit 1
    elif [[ $want_fail == true && $STATUS == false ]]; then
        sudo pkill -P ${WORKER_JOURNAL_PID}
        # use get_build_info to extract errors before picking the first
        errors=$(get_build_info ".errors" "$COMPOSE_START")
        ERROR_MSG=$(jq 'first(.[] | select(.id == "ManifestCreationFailed")) | .msg' <<< "${errors}")
        return
    else
        COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")
    fi

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
        COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

        # Is the compose finished?
        if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 5
    done

    # Kill the journal monitor
    sudo pkill -P ${WORKER_JOURNAL_PID}

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        redprint "Something went wrong with the compose. 😢"
        exit 1
    fi
}

check_result () {
    if [ ${#FAILED_MOUNTPOINTS[@]} -eq 0 ]; then
        greenprint "🎉 $1 scenario went as expected"
    else
        redprint "🔥 $1 scenario didn't go as expected. The following mountpoints were not present:"
        printf '%s\n' "${FAILED_MOUNTPOINTS[@]}"
        exit 1
    fi
}

##################################################
##
## Custom filesystems test - success case
##
##################################################

greenprint "🚀 Checking custom filesystems (success case)"

write_fs_blueprint() {
    tee "$BLUEPRINT_FILE" << EOF
name = "custom-filesystem"
description = "A base system with custom mountpoints"
version = "0.0.1"

[[customizations.filesystem]]
mountpoint = "/"
size = 2147483648

[[customizations.filesystem]]
mountpoint = "/var"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/var/log"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/var/log/audit"
size = "125 MiB"

[[customizations.filesystem]]
mountpoint = "/usr"
size = 4294967296

[[customizations.filesystem]]
mountpoint = "/tmp"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/var/tmp"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/home"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/home/shadowman"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/opt"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/srv"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/app"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/data"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/boot"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/boot/firmware"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/foobar"
size = 131072000
EOF
    EXPECTED_MOUNTPOINTS=(
        "/"
        "/var"
        "/var/log"
        "/var/log/audit"
        "/usr"
        "/tmp"
        "/var/tmp"
        "/home"
        "/home/shadowman"
        "/opt"
        "/srv"
        "/app"
        "/data"
        "/boot"
        "/boot/firmware"
        "/foobar"
    )
}

write_plain_blueprint() {
    tee "$BLUEPRINT_FILE" << EOF
name = "custom-filesystem"
description = "A base system with custom plain partitions"
version = "0.0.1"

[[customizations.disk.partitions]]
mountpoint = "/data"
fs_type = "ext4"
minsize = "1 GiB"

[[customizations.disk.partitions]]
mountpoint = "/home"
label = "home"
fs_type = "ext4"
minsize = "2 GiB"

[[customizations.disk.partitions]]
mountpoint = "/home/shadowman"
fs_type = "ext4"
minsize = "500 MiB"

[[customizations.disk.partitions]]
mountpoint = "/foo"
fs_type = "ext4"
minsize = "1 GiB"

[[customizations.disk.partitions]]
mountpoint = "/var"
fs_type = "xfs"
minsize = "4 GiB"

[[customizations.disk.partitions]]
mountpoint = "/opt"
fs_type = "ext4"
minsize = "1 GiB"

[[customizations.disk.partitions]]
mountpoint = "/media"
fs_type = "ext4"
minsize = "1 GiB"

[[customizations.disk.partitions]]
mountpoint = "/root"
fs_type = "ext4"
minsize = "1 GiB"

[[customizations.disk.partitions]]
mountpoint = "/srv"
fs_type = "xfs"
minsize = "1 GiB"

[[customizations.disk.partitions]]
fs_type = "swap"
minsize = "1 GiB"
EOF
    EXPECTED_MOUNTPOINTS=(
        "/data"
        "/home"
        "/home/shadowman"
        "/foo"
        "/var"
        "/opt"
        "/media"
        "/root"
        "/srv"
        "swap"
    )
}

write_lvm_blueprint() {
    tee "$BLUEPRINT_FILE" << EOF
name = "custom-filesystem"
description = "A base system with custom LVM partitioning"

[customizations.disk]
type = "gpt"

  [[customizations.disk.partitions]]
  mountpoint = "/data"
  minsize = "1 GiB"
  label = "data"
  fs_type = "ext4"

  [[customizations.disk.partitions]]
  type = "lvm"
  name = "testvg"
  minsize = 10_737_418_240

    [[customizations.disk.partitions.logical_volumes]]
    name = "homelv"
    mountpoint = "/home"
    label = "home"
    fs_type = "ext4"
    minsize = "2 GiB"

    [[customizations.disk.partitions.logical_volumes]]
    name = "shadowmanlv"
    mountpoint = "/home/shadowman"
    fs_type = "ext4"
    minsize = "5 GiB"

    [[customizations.disk.partitions.logical_volumes]]
    name = "foolv"
    mountpoint = "/foo"
    fs_type = "ext4"
    minsize = "1 GiB"

    [[customizations.disk.partitions.logical_volumes]]
    name = "usrlv"
    mountpoint = "/usr"
    fs_type = "ext4"
    minsize = "4 GiB"

    [[customizations.disk.partitions.logical_volumes]]
    name = "optlv"
    mountpoint = "/opt"
    fs_type = "ext4"
    minsize = 1_073_741_824

    [[customizations.disk.partitions.logical_volumes]]
    name = "medialv"
    mountpoint = "/media"
    fs_type = "ext4"
    minsize = 1_073_741_824

    [[customizations.disk.partitions.logical_volumes]]
    name = "roothomelv"
    mountpoint = "/root"
    fs_type = "ext4"
    minsize = "1 GiB"

    [[customizations.disk.partitions.logical_volumes]]
    name = "srvlv"
    mountpoint = "/srv"
    fs_type = "ext4"
    minsize = 1_073_741_824

    [[customizations.disk.partitions.logical_volumes]]
    name = "swap-lv"
    fs_type = "swap"
    minsize = "1 GiB"
EOF

    EXPECTED_MOUNTPOINTS=(
        "/data"
        "/home"
        "/home/shadowman"
        "/foo"
        "/usr"
        "/opt"
        "/media"
        "/root"
        "/srv"
        "swap"
    )
}

write_btrfs_blueprint() {
    tee "$BLUEPRINT_FILE" << EOF
name = "custom-filesystem"
description = "A base system with custom btrfs partitioning"

[[customizations.disk.partitions]]
type = "plain"
mountpoint = "/data"
minsize = 1_073_741_824
fs_type = "xfs"

[[customizations.disk.partitions]]
type = "btrfs"
minsize = "10 GiB"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-home"
  mountpoint = "/home"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-shadowman"
  mountpoint = "/home/shadowman"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-foo"
  mountpoint = "/foo"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-usr"
  mountpoint = "/usr"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-opt"
  mountpoint = "/opt"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-media"
  mountpoint = "/media"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-root"
  mountpoint = "/root"

  [[customizations.disk.partitions.subvolumes]]
  name = "subvol-srv"
  mountpoint = "/srv"

[[customizations.disk.partitions]]
type = "plain"
fs_type = "swap"
label = "swap-part"
minsize = "1 GiB"
EOF

    EXPECTED_MOUNTPOINTS=(
        "/data"
        "/home"
        "/home/shadowman"
        "/foo"
        "/usr"
        "/opt"
        "/media"
        "/root"
        "/srv"
        "swap"
    )
}

case "$CUSTOMIZATION_TYPE" in
    "filesystem")
        write_fs_blueprint
        ;;
    "disk-plain")
        write_plain_blueprint
        ;;
    "disk-lvm")
        write_lvm_blueprint
        ;;
    "disk-btrfs")
        write_btrfs_blueprint
        ;;
    *)
        redprint "Invalid value for CUSTOMIZATION_TYPE: ${CUSTOMIZATION_TYPE} - valid values are 'filesystem', 'disk-plain', 'disk-lvm', and 'disk-btrfs'"
        exit 1
        ;;
esac


build_image "$BLUEPRINT_FILE" custom-filesystem qcow2 false

# Download the image.
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "💬 Checking mountpoints"
if ! INFO="$(sudo osbuild-image-info "${IMAGE_FILENAME}")"; then
    echo "ERROR image-info failed, show last few kernel message to debug"
    dmesg | tail -n10
    exit 2
fi
FAILED_MOUNTPOINTS=()

for MOUNTPOINT in "${EXPECTED_MOUNTPOINTS[@]}"; do
  EXISTS=$(jq --arg m "$MOUNTPOINT" 'any(.fstab[] | .[] == $m; .)' <<< "${INFO}")
  if $EXISTS; then
    greenprint "INFO: mountpoint $MOUNTPOINT exists"
  else
    FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
  fi
done

# Check the result and pass scenario type
check_result "Passing"

# Clean compose and blueprints.
greenprint "🧼 Clean up osbuild-composer again"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete custom-filesystem > /dev/null

##################################################
##
## Custom filesystems test - fail case
##
##################################################

greenprint "🚀 Checking custom filesystems (fail case)"

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" << EOF
name = "custom-filesystem-fail"
description = "A base system with custom mountpoints"
version = "0.0.1"

[[customizations.filesystem]]
mountpoint = "/"
size = 2147483648

[[customizations.filesystem]]
mountpoint = "/etc"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/sys"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/proc"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/dev"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/run"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/bin"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/sbin"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/lib"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/lib64"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/lost+found"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/boot/efi"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/sysroot"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/usr/bin"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/var/run"
size = 131072000

[[customizations.filesystem]]
mountpoint = "/var/lock"
size = 131072000
EOF

build_image "$BLUEPRINT_FILE" custom-filesystem-fail qcow2 true

# Clear the test variable
FAILED_MOUNTPOINTS=()

greenprint "💬 Checking expected failures"
for MOUNTPOINT in '/etc' '/sys' '/proc' '/dev' '/run' '/bin' '/sbin' '/lib' '/lib64' '/lost+found' '/sysroot'; do
  if ! [[ $ERROR_MSG == *"$MOUNTPOINT"* ]]; then
    FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
  fi
done

if nvrGreaterOrEqual "osbuild-composer" "94"; then
  for MOUNTPOINT in '/usr/bin' '/var/run' '/var/lock'; do
    if ! [[ $ERROR_MSG == *"$MOUNTPOINT"* ]]; then
      FAILED_MOUNTPOINTS+=("$MOUNTPOINT")
    fi
  done
fi

# Check the result and pass scenario type
check_result "Failing"

# Clean compose and blueprints.
greenprint "🧼 Clean up osbuild-composer again"
sudo composer-cli blueprints delete custom-filesystem-fail > /dev/null

greenprint "🎉 All tests passed."
exit 0
