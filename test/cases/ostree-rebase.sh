#!/bin/bash
set -euo pipefail

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Get compose url if it's running on unsubscried RHEL
if [[ ${ID} == "rhel" ]] && ! sudo subscription-manager status; then
    source /usr/libexec/osbuild-composer-test/define-compose-url.sh
fi

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

function get_build_info() {
    key="$1"
    fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
    fi
    jq -r "${key}" "${fname}"
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
      <host mac='34:49:22:B0:83:30' name='vm-bios' ip='192.168.100.50'/>
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
        subject.isInGroup("adm")) {
            return polkit.Result.YES;
    }
});
EOF

# Set up variables.
TEST_UUID=$(uuidgen)
IMAGE_KEY="edge-${TEST_UUID}"
BIOS_GUEST_ADDRESS=192.168.100.50
UEFI_GUEST_ADDRESS=192.168.100.51
PROD_REPO_ADDRESS=192.168.200.1
PROD_REPO_URL="http://${PROD_REPO_ADDRESS}:8080/repo/"
ARTIFACTS="ci-artifacts"
CONTAINER_TYPE=edge-container
CONTAINER_FILENAME=container.tar
mkdir -p "${ARTIFACTS}"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
KS_FILE=${TEMPDIR}/ks.cfg
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)

case "${ID}-${VERSION_ID}" in
    "rhel-8.7")
        OSTREE_REF="rhel/8/${ARCH}/edge"
        OS_VARIANT="rhel8-unknown"
        # Use a stable installer image unless it's the nightly pipeline
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-8/RHEL-8/8.6.0/BaseOS/x86_64/os/"
        if [ "${NIGHTLY:=false}" == "true" ]; then
            BOOT_LOCATION="${COMPOSE_URL:-}/compose/BaseOS/x86_64/os/"
        fi
        PARENT_REF="rhel/8/${ARCH}/edge"
        ;;
    "rhel-9.1")
        OSTREE_REF="rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9-unknown"
        # Use a stable installer image unless it's the nightly pipeline
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-9/RHEL-9/9.0.0/BaseOS/x86_64/os/"
        if [ "${NIGHTLY:=false}" == "true" ]; then
            BOOT_LOCATION="${COMPOSE_URL:-}/compose/BaseOS/x86_64/os/"
        fi
        PARENT_REF="rhel/9/${ARCH}/edge"
        ;;
    "centos-8")
        OSTREE_REF="centos/8/${ARCH}/edge"
        OS_VARIANT="centos8"
        BOOT_LOCATION="http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/"
        PARENT_REF="centos/8/${ARCH}/edge"
        ;;
    "centos-9")
        OSTREE_REF="centos/9/${ARCH}/edge"
        OS_VARIANT="centos-stream9"
        BOOT_LOCATION="https://odcs.stream.centos.org/production/latest-CentOS-Stream/compose/BaseOS/x86_64/os/"
        PARENT_REF="centos/9/${ARCH}/edge"
        ;;
    *)
        echo "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac


# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.json

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
    # Stop watching the worker journal when exiting.
    trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

    # Start the compose.
    greenprint "🚀 Starting compose"
    if [ $# -eq 3 ]; then
        repo_url=$3
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" --url "$repo_url" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    elif [ $# -eq 4 ]; then
        repo_url=$3
        parent_ref=$4
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" --parent "$parent_ref" --url "$repo_url" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    else
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    fi
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
    # BIOS vm
    if [[ $(sudo virsh domstate "${IMAGE_KEY}-bios") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}-bios"
    fi
    sudo virsh undefine "${IMAGE_KEY}-bios"
    # Remove qcow2 file.
    sudo sudo virsh vol-delete --pool images "${IMAGE_KEY}-bios.qcow2"

    #UEFI vm
    if [[ $(sudo virsh domstate "${IMAGE_KEY}-uefi") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}-uefi"
    fi
    sudo virsh undefine "${IMAGE_KEY}-uefi" --nvram
    # Remove qcow2 file.
    sudo sudo virsh vol-delete --pool images "${IMAGE_KEY}-uefi.qcow2"

    # Remove all containers and images if exist
    sudo podman system reset --force

    # Remomve tmp dir.
    sudo rm -rf "$TEMPDIR"
}

# Test result checking
check_result () {
    greenprint "🎏 Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        greenprint "❌ Failed"
        clean_up
        exit 1
    fi
}

# Prepare stage repo network
greenprint "🔧 Prepare stage repo network"
sudo podman network inspect edge >/dev/null 2>&1 || sudo podman network create --driver=bridge --subnet=192.168.200.0/24 --gateway=192.168.200.254 edge

##########################################################
##
## Build edge-container image and start it in podman
##
##########################################################

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "container"
description = "A base rhel-edge container image"
version = "0.0.1"
modules = []
groups = []

[[packages]]
name = "python3"
version = "*"

[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/admin/"
groups = ["wheel"]
EOF

greenprint "📄 container blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing container blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve container

# Build container image.
build_image container "${CONTAINER_TYPE}"

# Download the image
greenprint "📥 Downloading the container image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

# Clear stage repo running env
greenprint "🧹 Clearing stage repo running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

# Deal with stage repo image
greenprint "🗜 Starting container"
IMAGE_FILENAME="${COMPOSE_ID}-${CONTAINER_FILENAME}"
sudo podman pull "oci-archive:${IMAGE_FILENAME}"
sudo podman images
# Run edge stage repo
greenprint "🛰 Running edge stage repo"
# Get image id to run image
EDGE_IMAGE_ID=$(sudo podman images --filter "dangling=true" --format "{{.ID}}")
sudo podman run -d --name rhel-edge --network edge --ip "$PROD_REPO_ADDRESS" "$EDGE_IMAGE_ID"
# Clear image file
sudo rm -f "$IMAGE_FILENAME"

# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Clean compose and blueprints.
greenprint "🧽 Clean up container blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete container > /dev/null

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create bios vm qcow2 file for virt install.
greenprint "Create bios vm qcow2 file for virt install"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-bios.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

# Write kickstart file for ostree image installation.
greenprint "Generate kickstart file"
tee "$KS_FILE" > /dev/null << STOPHERE
text
network --bootproto=dhcp --device=link --activate --onboot=on

zerombr
clearpart --all --initlabel --disklabel=msdos
autopart --nohome --noswap --type=plain
rootpw --lock --iscrypted locked
ostreesetup --nogpg --osname=rhel-edge --remote=rhel-edge --url=${PROD_REPO_URL} --ref=${OSTREE_REF}
poweroff

%post --log=/var/log/anaconda/post-install.log --erroronfail
# no sudo password for SSH user
echo -e 'admin\tALL=(ALL)\tNOPASSWD: ALL' >> /etc/sudoers
%end
STOPHERE

# Install ostree image via anaconda on BIOS vm.
greenprint "Install ostree image via anaconda on BIOS vm"
sudo virt-install  --initrd-inject="${KS_FILE}" \
                   --extra-args="inst.ks=file:/ks.cfg console=ttyS0,115200" \
                   --name="${IMAGE_KEY}-bios"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 3072 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-type linux \
                   --os-variant ${OS_VARIANT} \
                   --location "${BOOT_LOCATION}" \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "Start VM"
sudo virsh start "${IMAGE_KEY}-bios"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $BIOS_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

# Create uefi vm qcow2 file for virt install.
greenprint "Create uefi vm qcow2 file for virt install"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-uefi.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

# Install ostree image via anaconda on UEFI vm.
greenprint "Install ostree image via anaconda on UEFI vm"
sudo virt-install  --initrd-inject="${KS_FILE}" \
                   --extra-args="inst.ks=file:/ks.cfg console=ttyS0,115200" \
                   --name="${IMAGE_KEY}-uefi"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 3072 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:31 \
                   --os-type linux \
                   --os-variant ${OS_VARIANT} \
                   --location "${BOOT_LOCATION}" \
                   --boot uefi,loader_ro=yes,loader_type=pflash,nvram_template=/usr/share/edk2/ovmf/OVMF_VARS.fd,loader_secure=no \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "Start VM"
sudo virsh start "${IMAGE_KEY}-uefi"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $UEFI_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

##################################################################
##
## Build upgrade image with new ref
##
##################################################################

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "upgrade"
description = "An upgrade rhel-edge container image"
version = "0.0.2"
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
home = "/home/admin/"
key = "${SSH_KEY_PUB}"
groups = ["wheel"]

[customizations.kernel]
name = "kernel-rt"
EOF

greenprint "📄 upgrade blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing upgrade blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve upgrade

# Build upgrade image.
OSTREE_REF="test/rhel/x/${ARCH}/edge"
build_image upgrade "${CONTAINER_TYPE}" "$PROD_REPO_URL" "$PARENT_REF"

# Download the image
greenprint "📥 Downloading the upgrade image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

# Clear stage repo running env
greenprint "🧹 Clearing stage repo running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

# Deal with stage repo container
greenprint "🗜 Extracting image"
IMAGE_FILENAME="${COMPOSE_ID}-${CONTAINER_FILENAME}"
sudo podman pull "oci-archive:${IMAGE_FILENAME}"
sudo podman images
# Clear image file
sudo rm -f "$IMAGE_FILENAME"

# Run edge stage repo
greenprint "🛰 Running edge stage repo"
# Get image id to run image
EDGE_IMAGE_ID=$(sudo podman images --filter "dangling=true" --format "{{.ID}}")
sudo podman run -d --name rhel-edge --network edge --ip "$PROD_REPO_ADDRESS" "$EDGE_IMAGE_ID"
# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Get ostree commit value.
greenprint "🕹  Get ostree upgrade commit value"
UPGRADE_HASH=$(curl "${PROD_REPO_URL}refs/heads/${OSTREE_REF}")

# Clean compose and blueprints.
greenprint "🧽 Clean up upgrade blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete upgrade > /dev/null

# Rebase with new ref on BIOS vm
greenprint "🗳 Rebase with new ref on BIOS vm"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${BIOS_GUEST_ADDRESS} "sudo rpm-ostree rebase rhel-edge:${OSTREE_REF}"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${BIOS_GUEST_ADDRESS} "nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $BIOS_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check ostree upgrade result
check_result

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${BIOS_GUEST_ADDRESS}
[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=admin
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=rhel-edge -e ostree_commit="${UPGRADE_HASH}" -e ostree_ref="rhel-edge:${OSTREE_REF}" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Rebase with new ref on UEFI vm
greenprint "🗳 Rebase with new ref on UEFI vm"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${UEFI_GUEST_ADDRESS} "sudo rpm-ostree rebase rhel-edge:${OSTREE_REF}"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${UEFI_GUEST_ADDRESS} "nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $UEFI_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check ostree upgrade result
check_result

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${UEFI_GUEST_ADDRESS}
[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=admin
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=rhel-edge -e ostree_commit="${UPGRADE_HASH}" -e ostree_ref="rhel-edge:${OSTREE_REF}" -e skip_rollback_test="true" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0
