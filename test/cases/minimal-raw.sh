#!/bin/bash
set -euo pipefail

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"
}
trap cleanup EXIT

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Start firewalld
greenprint "Start firewalld"
sudo systemctl enable --now firewalld

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
  <bridge name='integration' zone='trusted' stp='on' delay='0'/>
  <mac address='52:54:00:36:46:ef'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
      <host mac='34:49:22:B0:83:31' name='vm-uefi' ip='192.168.100.51'/>
    </dhcp>
  </ip>
</network>
EOF
if ! sudo virsh net-info integration > /dev/null 2>&1; then
    sudo virsh net-define /tmp/integration.xml
fi
if [[ $(sudo virsh net-info integration | grep 'Active' | awk '{print $2}') == 'no' ]]; then
    sudo virsh net-start integration
fi

# Allow anyone in the wheel group to talk to libvirt.
greenprint "🚪 Allowing users in wheel group to talk to libvirt"
sudo tee /etc/polkit-1/rules.d/50-libvirt.rules > /dev/null << EOF
polkit.addRule(function(action, subject) {
    if (action.id == "org.libvirt.unix.manage" &&
        subject.isInGroup("whell")) {
            return polkit.Result.YES;
    }
});
EOF

# Set up variables.
TEST_UUID=$(uuidgen)
IMAGE_KEY="minimal-raw-${TEST_UUID}"
UEFI_GUEST_ADDRESS=192.168.100.51
MINIMAL_RAW_TYPE=minimal-raw
MINIMAL_RAW_DECOMPRESSED=disk.raw
MINIMAL_RAW_FILENAME=disk.raw.xz
BOOT_ARGS="uefi"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)
EDGE_USER_PASSWORD=foobar

case "${ID}-${VERSION_ID}" in
    "rhel-8."*)
        OS_VARIANT="rhel8-unknown"
        ;;
    "rhel-9."*)
        OS_VARIANT="rhel9-unknown"
        ;;
    "centos-8")
        OS_VARIANT="centos-stream8"
        ;;
    "centos-9")
        OS_VARIANT="centos-stream9"
        BOOT_ARGS="uefi,firmware.feature0.name=secure-boot,firmware.feature0.enabled=no"
        ;;
    "fedora-37")
        OS_VARIANT="fedora37"
        ;;
    "fedora-38")
        OS_VARIANT="fedora-unknown"
        ;;
    "fedora-39")
        OS_VARIANT="fedora-rawhide"
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=osbuild-${ID}-${VERSION_ID}-minimal-raw-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=osbuild-${ID}-${VERSION_ID}-minimal-raw-${COMPOSE_ID}.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL" -C "${TEMPDIR}"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${TEMPDIR}"/"${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Build ostree image.
build_image() {
    blueprint_name=$1
    image_type=$2

    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!

    # Start the compose.
    greenprint "🚀 Starting compose"
    sudo composer-cli --json compose start "$blueprint_name" "$image_type" | tee "$COMPOSE_START"

    COMPOSE_ID=$(jq -r '.[0].body.build_id' "$COMPOSE_START")

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

    # Kill the journal monitor
    sudo pkill -P ${WORKER_JOURNAL_PID}

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        redprint "Something went wrong with the compose. 😢"
        exit 1
    fi
}

# Wait for the ssh server up to be.
wait_for_ssh_up () {
    SSH_STATUS=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@"${1}" '/bin/bash -c "echo -n READY"')
    if [[ $SSH_STATUS == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

# Clean up our mess.
clean_up () {
    greenprint "🧼 Cleaning up"

    # Clear vm
    if [[ $(sudo virsh domstate "${IMAGE_KEY}-uefi") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}-uefi"
    fi
    sudo virsh undefine "${IMAGE_KEY}-uefi" --nvram
    # Remove qcow2 file.
    sudo virsh vol-delete --pool images "${IMAGE_KEY}-uefi.qcow2"

    # Remomve tmp dir.
    sudo rm -rf "$TEMPDIR"
}

# Test result checking
check_result () {
    greenprint "🎏 Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        redprint "❌ Failed"
        clean_up
        exit 1
    fi
}

############################################################
##
## Build mininal-raw image
##
############################################################

# Write a blueprint for minimal-raw image image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "minimal-raw"
description = "A minimal raw image"
version = "0.0.1"
modules = []
groups = []

[[packages]]
name = "python3"
version = "*"

[[packages]]
name = "wget"
version = "*"

[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/admin/"
groups = ["wheel"]
EOF

greenprint "📄 minimal raw image blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing minimal-raw image blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve minimal-raw

# Build minimal-raw image.
build_image minimal-raw "${MINIMAL_RAW_TYPE}"

# Download the image
greenprint "📥 Downloading the minimal-raw image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

greenprint "Extracting and converting the raw image to a qcow2 file"
MINIMAL_RAW_FILENAME="${COMPOSE_ID}-${MINIMAL_RAW_FILENAME}"
sudo xz -d "${MINIMAL_RAW_FILENAME}"
LIBVIRT_IMAGE_PATH_UEFI=/var/lib/libvirt/images/"${IMAGE_KEY}-uefi.qcow2"
sudo qemu-img convert -f raw "${COMPOSE_ID}-${MINIMAL_RAW_DECOMPRESSED}" -O qcow2 "$LIBVIRT_IMAGE_PATH_UEFI"
# Remove raw file
sudo rm -f "$MINIMAL_RAW_FILENAME"

# Clean compose and blueprints.
greenprint "🧹 Clean up minimal-raw blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete minimal-raw > /dev/null

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# create artifacts folder
ARTIFACTS="${ARTIFACTS:=/tmp/artifacts}"
mkdir -p "${ARTIFACTS}"

VIRT_LOG="$ARTIFACTS/minimal-raw-virt-install-console.log"
touch "$VIRT_LOG"
sudo chown qemu:qemu "$VIRT_LOG"

##################################################################
##
## Install and test minimal-raw image (UEFI)
##
##################################################################
greenprint "💿 Installing minimal-raw image on UEFI VM"
sudo virt-install  --name="${IMAGE_KEY}-uefi"\
                   --disk path="${LIBVIRT_IMAGE_PATH_UEFI}",format=qcow2 \
                   --ram 3072 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:31 \
                   --os-variant ${OS_VARIANT} \
                   --boot ${BOOT_ARGS} \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --import \
                   --noreboot \
                   --console pipe,source.path="$VIRT_LOG"


# Start VM.
greenprint "💻 Start UEFI VM"
sudo virsh start "${IMAGE_KEY}-uefi"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $UEFI_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

# Add instance IP address into /etc/ansible/hosts
tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${UEFI_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=admin
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes 
ansible_become_method=sudo
ansible_become_pass=${EDGE_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e download_node="$DOWNLOAD_NODE" /usr/share/tests/osbuild-composer/ansible/check-minimal.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0
