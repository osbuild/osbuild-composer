#!/bin/bash
set -euo pipefail

#
# tests that guest images are buildable using composer-cli and and verifies
# they boot with cloud-init using libvirt
#

OSBUILD_COMPOSER_TEST_DATA=/usr/share/tests/osbuild-composer/

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Take the image type passed to the script or use qcow2 by default if nothing
# was passed.
IMAGE_TYPE=${1:-qcow2}
# Take the boot type passed to the script or use BIOS by default if nothing
# was passed.
BOOT_TYPE=${2:-bios}
# Take the image from the url passes to the script or build it by default if nothing
LIBVIRT_IMAGE_URL=${3:-""}
# When downloading the image, if provided, use this CA bundle, or skip verification
LIBVIRT_IMAGE_URL_CA_BUNDLE=${4:-""}

# Select the file extension based on the image that we are building.
IMAGE_EXTENSION=$IMAGE_TYPE
if [[ $IMAGE_TYPE == 'openstack' ]]; then
    IMAGE_EXTENSION=qcow2
fi

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

# Start libvirtd and test it.
greenprint "🚀 Starting libvirt daemon"
sudo systemctl start libvirtd
sudo virsh list --all > /dev/null

# Set a customized dnsmasq configuration for libvirt so we always get the
# same address on bootup.
sudo tee /tmp/integration.xml > /dev/null << EOF
<network>
  <name>integration</name>
  <uuid>1c8fe98c-b53a-4ca4-bbdb-deb0f26b3579</uuid>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='integration' stp='on' delay='0'/>
  <mac address='52:54:00:36:46:ef'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
      <host mac='34:49:22:B0:83:30' name='vm' ip='192.168.100.50'/>
    </dhcp>
  </ip>
</network>
EOF
if ! sudo virsh net-info integration > /dev/null 2>&1; then
    sudo virsh net-define /tmp/integration.xml
    sudo virsh net-start integration
fi

# Allow anyone in the wheel group to talk to libvirt.
greenprint "🚪 Allowing users in wheel group to talk to libvirt"
WHEEL_GROUP=wheel
if [[ $ID == rhel ]]; then
    WHEEL_GROUP=adm
fi
sudo tee /etc/polkit-1/rules.d/50-libvirt.rules > /dev/null << EOF
polkit.addRule(function(action, subject) {
    if (action.id == "org.libvirt.unix.manage" &&
        subject.isInGroup("${WHEEL_GROUP}")) {
            return polkit.Result.YES;
    }
});
EOF

# Set up variables.
TEST_UUID=$(uuidgen)
IMAGE_KEY=osbuild-composer-qemu-test-${TEST_UUID}
INSTANCE_ADDRESS=192.168.100.50

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa

VIRT_LOG="$ARTIFACTS/libvirt_tests-virt-install-console.log"
if [ ! -f "$VIRT_LOG" ]; then
    touch "$VIRT_LOG"
fi
sudo chown qemu:qemu "$VIRT_LOG"

# Check for the smoke test file on the AWS instance that we start.
smoke_test_check () {
    # Ensure the ssh key has restricted permissions.
    SSH_OPTIONS=(-o StrictHostKeyChecking=no -o ConnectTimeout=5)
    SMOKE_TEST=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" redhat@"${1}" 'cat /etc/smoke-test.txt')
    if [[ $SMOKE_TEST == smoke-test ]]; then
        echo 1
    else
        echo 0
    fi
}

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${IMAGE_TYPE}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${IMAGE_TYPE}.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Current $PWD is inside /tmp, there may not be enough space for an image.
# Let's use a bigger temporary directory for this operation.
BIG_TEMP_DIR=/var/lib/osbuild-composer-tests
sudo rm -rf "${BIG_TEMP_DIR}" || true
sudo mkdir "${BIG_TEMP_DIR}"

if [ -z "${LIBVIRT_IMAGE_URL}" ]; then
    # Write a basic blueprint for our image.
    tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "bp"
description = "A base system"
version = "0.0.1"

# Related RHBZ#2065734
[[packages]]
name = "ipa-client"
version = "*"
EOF

    # Prepare the blueprint for the compose.
    greenprint "📋 Preparing blueprint"
    sudo composer-cli blueprints push "$BLUEPRINT_FILE"
    sudo composer-cli blueprints depsolve bp

    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!
    # Stop watching the worker journal when exiting.
    trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

    # Start the compose
    greenprint "🚀 Starting compose"
    sudo composer-cli --json compose start bp "$IMAGE_TYPE" | tee "$COMPOSE_START"
    COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

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

    # Capture the compose logs from osbuild.
    greenprint "💬 Getting compose log and metadata"
    get_compose_log "$COMPOSE_ID"
    get_compose_metadata "$COMPOSE_ID"

    # Kill the journal monitor immediately and remove the trap
    sudo pkill -P ${WORKER_JOURNAL_PID}
    trap - EXIT

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi

    # Download the image.
    greenprint "📥 Downloading the image"

    pushd "${BIG_TEMP_DIR}"
        sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
        IMAGE_FILENAME=$(basename "$(find . -maxdepth 1 -type f -name "*.${IMAGE_EXTENSION}")")
        LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.${IMAGE_EXTENSION}
        sudo mv "$IMAGE_FILENAME" "$LIBVIRT_IMAGE_PATH"
    popd
else
    pushd "${BIG_TEMP_DIR}"
    LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.${IMAGE_EXTENSION}
    if [ -n "${LIBVIRT_IMAGE_URL_CA_BUNDLE}" ]; then
        if [ "${LIBVIRT_IMAGE_URL_CA_BUNDLE}" == "skip" ]; then
            sudo curl -o "${LIBVIRT_IMAGE_PATH}" -k "${LIBVIRT_IMAGE_URL}"
        else
            sudo curl -o "${LIBVIRT_IMAGE_PATH}" --cacert "${LIBVIRT_IMAGE_URL_CA_BUNDLE}" "${LIBVIRT_IMAGE_URL}"
        fi
    else
        sudo curl -o "${LIBVIRT_IMAGE_PATH}" "${LIBVIRT_IMAGE_URL}"
    fi
    popd
fi

# Prepare cloud-init data.
CLOUD_INIT_DIR=$(mktemp -d)
cp "${OSBUILD_COMPOSER_TEST_DATA}"/cloud-init/meta-data "${CLOUD_INIT_DIR}"/
cp "${SSH_DATA_DIR}"/user-data "${CLOUD_INIT_DIR}"/

# Set up a cloud-init ISO.
greenprint "💿 Creating a cloud-init ISO"
CLOUD_INIT_PATH=/var/lib/libvirt/images/seed.iso
rm -f $CLOUD_INIT_PATH
pushd "$CLOUD_INIT_DIR"
    sudo mkisofs -o $CLOUD_INIT_PATH -V cidata \
        -r -J user-data meta-data > /dev/null 2>&1
popd

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Run virt-install to import the QCOW and boot it.
greenprint "🚀 Booting the image with libvirt"
if [[ $ARCH == 'ppc64le' ]]; then
    # ppc64le has some machine quirks that must be worked around.
    sudo virt-install \
        --name "$IMAGE_KEY" \
        --memory 2048 \
        --vcpus 2 \
        --disk path="${LIBVIRT_IMAGE_PATH}" \
        --disk path=${CLOUD_INIT_PATH},device=cdrom \
        --import \
        --os-variant rhel8-unknown \
        --noautoconsole \
        --network network=integration,mac=34:49:22:B0:83:30 \
        --qemu-commandline="-machine pseries,cap-cfpc=broken,cap-sbbc=broken,cap-ibs=broken,cap-ccf-assist=off,cap-large-decr=off" \
        --console pipe,source.path="$VIRT_LOG"
elif [[ $ARCH == 's390x' ]]; then
    # Our s390x machines are highly constrained on resources.
    sudo virt-install \
        --name "$IMAGE_KEY" \
        --memory 512 \
        --vcpus 1 \
        --disk path="${LIBVIRT_IMAGE_PATH}" \
        --disk path=${CLOUD_INIT_PATH},device=cdrom \
        --import \
        --os-variant rhel8-unknown \
        --noautoconsole \
        --network network=integration,mac=34:49:22:B0:83:30 \
        --console pipe,source.path="$VIRT_LOG"
else
    case "${ID}-${VERSION_ID}" in
        "rhel-8"*)
             # was 1024 before 8.9
            MIN_RAM="1536"
            NVRAM_TEMPLATE="nvram_template=/usr/share/edk2/ovmf/OVMF_VARS.fd"
            OS_VARIANT="rhel8-unknown"
            ;;
        "centos-8")
            MIN_RAM="1536"
            NVRAM_TEMPLATE="nvram_template=/usr/share/edk2/ovmf/OVMF_VARS.fd"
            OS_VARIANT="centos-stream8"
            ;;
        "rhel-9"*)
            MIN_RAM="1536"
            NVRAM_TEMPLATE=""
            OS_VARIANT="rhel9-unknown"
            ;;
        "centos-9")
            MIN_RAM="1536"
            # Disable secure boot for CS9 due to bug bz#2108646
            NVRAM_TEMPLATE="firmware.feature0.name=secure-boot,firmware.feature0.enabled=no"
            OS_VARIANT="centos-stream9"
            ;;
        # TODO: Update the variants once available
        "rhel-10"*)
            MIN_RAM="1536"
            NVRAM_TEMPLATE=""
            OS_VARIANT="rhel9-unknown"
            ;;
        "centos-10")
            MIN_RAM="1536"
            # Disable secure boot for CS9 due to bug bz#2108646
            NVRAM_TEMPLATE="firmware.feature0.name=secure-boot,firmware.feature0.enabled=no"
            OS_VARIANT="centos-stream9"
            ;;
        "fedora"*)
            MIN_RAM="1536"
            NVRAM_TEMPLATE=""
            OS_VARIANT="fedora-unknown"
            ;;
        *)
            echo "unsupported distro: ${ID}-${VERSION_ID}"
            exit 1;;
    esac

    # Both aarch64 and x86_64 support hybrid boot
    if [[ $BOOT_TYPE == 'uefi' ]]; then
        sudo virt-install \
            --name "$IMAGE_KEY" \
            --memory ${MIN_RAM} \
            --vcpus 2 \
            --disk path="${LIBVIRT_IMAGE_PATH}" \
            --disk path=${CLOUD_INIT_PATH},device=cdrom \
            --import \
            --os-variant "${OS_VARIANT}" \
            --noautoconsole \
            --boot uefi,"${NVRAM_TEMPLATE}" \
            --network network=integration,mac=34:49:22:B0:83:30 \
            --console pipe,source.path="$VIRT_LOG"
    else
        sudo virt-install \
            --name "$IMAGE_KEY" \
            --memory ${MIN_RAM} \
            --vcpus 2 \
            --disk path="${LIBVIRT_IMAGE_PATH}" \
            --disk path=${CLOUD_INIT_PATH},device=cdrom \
            --import \
            --os-variant "${OS_VARIANT}" \
            --noautoconsole \
            --network network=integration,mac=34:49:22:B0:83:30 \
            --console pipe,source.path="$VIRT_LOG"
    fi
fi

# Set a number of maximum loops to check for our smoke test file via ssh.
case $ARCH in
    s390x)
        # s390x needs more time to boot its VM.
        MAX_LOOPS=60
        ;;
    *)
        MAX_LOOPS=30
        ;;
esac

# Check for our smoke test file.
greenprint "🛃 Checking for smoke test file in VM"
# shellcheck disable=SC2034  # Unused variables left for readability
for LOOP_COUNTER in $(seq 0 ${MAX_LOOPS}); do
    RESULTS="$(smoke_test_check $INSTANCE_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "Smoke test passed! 🥳"
        break
    fi
    echo "Machine is not ready yet, retrying connection."
    sleep 10
done
# additional tests for regressions
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o ConnectTimeout=5)
# simple check if manual pages are installed, rhbz#2004401
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" redhat@"$INSTANCE_ADDRESS" 'man man > /dev/null'
# check for grubenv presence and if  grubby command exits with 0, rhbz#2003038
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" redhat@"$INSTANCE_ADDRESS" 'sudo ls /boot/grub2/grubenv > /dev/null'
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" redhat@"$INSTANCE_ADDRESS" 'sudo grubby --set-default-index=0 > /dev/null'

# Clean up our mess.
greenprint "🧼 Cleaning up"
sudo virsh destroy "${IMAGE_KEY}"
if [[ $ARCH == aarch64 || $BOOT_TYPE == 'uefi' ]]; then
    sudo virsh undefine "${IMAGE_KEY}" --nvram
else
    sudo virsh undefine "${IMAGE_KEY}"
fi
sudo rm -f "$LIBVIRT_IMAGE_PATH" $CLOUD_INIT_PATH

if [ -z "${LIBVIRT_IMAGE_URL}" ]; then
    # Also delete the compose so we don't run out of disk space
    sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
fi

# Use the return code of the smoke test to determine if we passed or failed.
if [[ $RESULTS == 1 ]]; then
  greenprint "💚 Success"
else
  greenprint "❌ Failed"
  exit 1
fi

exit 0
