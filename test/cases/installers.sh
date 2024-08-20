#!/bin/bash
set -euo pipefail

#
# This test builds a image-installer iso which is then used for OS installation. 
# To do so, it creates an image-installer iso, then modifies the kickstart so it 
# can be installed automatically, installs the VM with virt-install waits for it 
# to boot and then run some test using ansible to check if everything is as expected
#

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

isomount=$(mktemp -d)
kspath=$(mktemp -d)
cleanup() {
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"

    sudo umount -v "${isomount}" || echo
    rmdir -v "${isomount}"
    rm -rv "${kspath}"
}
trap cleanup EXIT

# modify existing kickstart by prepending and appending commands
function modksiso {
    sudo dnf install -y lorax  # for mkksiso

    iso="$1"
    newiso="$2"

    echo "Mounting ${iso} -> ${isomount}"
    sudo mount -v -o ro "${iso}" "${isomount}"

    ksfiles=("${isomount}"/*.ks)
    ksfile="${ksfiles[0]}"  # there shouldn't be more than one anyway
    echo "Found kickstart file ${ksfile}"

    ksbase=$(basename "${ksfile}")
    newksfile="${kspath}/${ksbase}"
    oldks=$(cat "${ksfile}")
    echo "Preparing modified kickstart file"
    cat > "${newksfile}" << EOFKS
text --non-interactive
zerombr
clearpart --all --initlabel --disklabel=gpt
autopart --noswap --type=plain
network --bootproto=dhcp --device=link --activate --onboot=on
${oldks}
poweroff

%post --log=/var/log/anaconda/post-install.log --erroronfail

# no sudo password for user admin
echo -e 'admin\tALL=(ALL)\tNOPASSWD: ALL' >> /etc/sudoers
%end
EOFKS

    echo "Writing new ISO"
    if nvrGreaterOrEqual "lorax" "34.9.18"; then
        sudo mkksiso -c "console=ttyS0,115200" --ks "${newksfile}" "${iso}" "${newiso}"
    else
        sudo mkksiso -c "console=ttyS0,115200" "${newksfile}" "${iso}" "${newiso}"
    fi

    echo "==== NEW KICKSTART FILE ===="
    cat "${newksfile}"
    echo "============================"
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
case "${ID}-${VERSION_ID}" in
    rhel-8*)
        OS_VARIANT="rhel8-unknown"
        ;;
    rhel-9*)
        OS_VARIANT="rhel9-unknown"
        ;;
    rhel-10*)
        # TODO: change to rhel10-unknown once it's available
        OS_VARIANT="rhel-unknown"
        ;;
    centos-8)
        OS_VARIANT="centos8"
        ;;
    centos-9)
        OS_VARIANT="centos-stream9"
        ;;
    centos-10)
        # TODO: change to centos-stream10 once it's available
        OS_VARIANT="rhel-unknown"
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac
TEST_UUID=$(uuidgen)
SSH_USER="admin"
IMAGE_KEY="osbuild-composer-installer-test-${TEST_UUID}"
GUEST_ADDRESS=192.168.100.50

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

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

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    greenprint "Saving compose log for ${COMPOSE_ID} to artifacts"
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    greenprint "Saving manifest for ${COMPOSE_ID}"
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

# Build an installer
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
    COMPOSE_ID=$(get_build_info ".build_id" "${COMPOSE_START}")

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
        COMPOSE_STATUS=$(get_build_info ".queue_status" "${COMPOSE_INFO}")

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

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        redprint "Something went wrong with the compose. 😢"
        exit 1
    fi

    # Kill the journal monitor
    sudo pkill -P ${WORKER_JOURNAL_PID}
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
    if [[ $(sudo virsh domstate "${IMAGE_KEY}") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}"
    fi
    sudo virsh undefine "${IMAGE_KEY}" --nvram
    # Remove qcow2 file.
    sudo rm -f "$LIBVIRT_IMAGE_PATH"

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

#############################
##
## installer image building
##
############################

# Write a blueprint for installer image.
# The tar base image is very lean, so let's add a bunch of packages to make it
# usable (adding most of the packages from the qcow2 image plus some for fun)
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "installer"
description = "An installer image"
version = "0.0.1"
modules = []
groups = []

[[packages]]
name = "vim-enhanced"
version = "*"

[[packages]]
name = "tmux"
version = "*"

[[packages]]
name = "sudo"
version = "*"

[[packages]]
name = "openssh-server"
version = "*"

[[packages]]
name = "python3"
version = "*"

[[customizations.group]]
name = "testers"

[[customizations.user]]
name = "${SSH_USER}"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/${SSH_USER}/"
groups = ["wheel", "testers"]

[customizations.installer]
unattended = true
sudo-nopasswd = ["admin"]
EOF

greenprint "📄 installer blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve installer

# Build installer image.
build_image installer image-installer

# Download the image
greenprint "📥 Downloading the installer image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-installer.iso"
greenprint "🖥 Modify kickstart file and create new ISO"

# in nightly pipelines the feature wont be available for a while, so the
# customizations will have no effect and we need to modify the kickstart file
# on the ISO
if [[ "${NIGHTLY:=false}" == "true" ]] && ! nvrGreaterOrEqual "osbuild-composer" "103"; then
    modksiso "${ISO_FILENAME}" "/var/lib/libvirt/images/${ISO_FILENAME}"
    sudo rm "${ISO_FILENAME}"
else
    sudo mv "${ISO_FILENAME}" "/var/lib/libvirt/images/${ISO_FILENAME}"
fi

# Clean compose and blueprints.
greenprint "🧹 Clean up installer blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete installer > /dev/null

########################################################
##
## install image with installer(ISO)
##
########################################################

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create qcow2 files for virt install.
greenprint "🖥 Create qcow2 files for virt install"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

#########################
##
## Install and test image
##
#########################
# Install image via anaconda.

VIRT_LOG="$ARTIFACTS/installers-sh-virt-install-console.log"
touch "$VIRT_LOG"
sudo chown qemu:qemu "$VIRT_LOG"

greenprint "💿 Install image via installer(ISO) on VM"
sudo virt-install  --name="${IMAGE_KEY}"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 2048 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot \
                   --console pipe,source.path="$VIRT_LOG"


# Start VM.
greenprint "💻 Start VM"
sudo virsh start "${IMAGE_KEY}"

# Waiting for SSH
greenprint "🛃 Checking if SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[guest]
${GUEST_ADDRESS}

[guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=admin
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory /usr/share/tests/osbuild-composer/ansible/check_install.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0
