#!/bin/bash
set -euo pipefail

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

# Do not use httpd, but container
sudo systemctl disable --now httpd

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
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
sudo tee /etc/polkit-1/rules.d/50-libvirt.rules > /dev/null << EOF
polkit.addRule(function(action, subject) {
    if (action.id == "org.libvirt.unix.manage" &&
        subject.isInGroup("adm")) {
            return polkit.Result.YES;
    }
});
EOF

# Set up variables.
OSBUILD_COMPOSER_TEST_DATA=/usr/share/tests/osbuild-composer/
OSTREE_REF="test/rhel/8/${ARCH}/edge"
OS_VARIANT="rhel8-unknown"
TEST_UUID=$(uuidgen)
IMAGE_KEY="osbuild-composer-ostree-test-${TEST_UUID}"
GUEST_ADDRESS=192.168.100.50
URL=http://192.168.100.1/repo

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
KS_FILE=${TEMPDIR}/ks.cfg
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_KEY=${OSBUILD_COMPOSER_TEST_DATA}keyring/id_rsa

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    tar -xf "$TARBALL" -C "${TEMPDIR}"
    rm -f "$TARBALL"

    # Move the JSON file into place.
    cat "${TEMPDIR}"/"${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
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
    if [ $# -eq 3 ]; then
        repo_url=$3
        sudo curl --silent --header "Content-Type: application/json" --unix-socket /run/weldr/api.socket http://localhost/api/v1/compose --data "{
            \"blueprint_name\": \"$blueprint_name\",
            \"compose_type\": \"$image_type\",
            \"ostree\": {
                \"url\": \"$repo_url\",
                \"ref\": \"$OSTREE_REF\"
            }
        }" | tee "$COMPOSE_START"
    else
        # Test ref begining with /
        sudo curl --silent --header "Content-Type: application/json" --unix-socket /run/weldr/api.socket http://localhost/api/v1/compose --data "{
            \"blueprint_name\": \"$blueprint_name\",
            \"compose_type\": \"$image_type\",
            \"ostree\": {
                \"ref\": \"$OSTREE_REF\"
            }
        }" | tee "$COMPOSE_START"
    fi
    COMPOSE_ID=$(jq -r '.build_id' "$COMPOSE_START")

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
        COMPOSE_STATUS=$(jq -r '.queue_status' "$COMPOSE_INFO")

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
    sudo virsh destroy "${IMAGE_KEY}"
    if [[ $ARCH == aarch64 ]]; then
        sudo virsh undefine "${IMAGE_KEY}" --nvram
    else
        sudo virsh undefine "${IMAGE_KEY}"
    fi
    # Remove qcow2 file.
    sudo rm -f "$LIBVIRT_IMAGE_PATH"

    # Remove any status containers if exist
    sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
    # Remove all images
    sudo podman rmi -f -a

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

##########################################################
##
## rhel-edge container image for building installer image
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
name = "python36"
version = "*"

[customizations.kernel]
name = "kernel-rt"

[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC61wMCjOSHwbVb4VfVyl5sn497qW4PsdQ7Ty7aD6wDNZ/QjjULkDV/yW5WjDlDQ7UqFH0Sr7vywjqDizUAqK7zM5FsUKsUXWHWwg/ehKg8j9xKcMv11AkFoUoujtfAujnKODkk58XSA9whPr7qcw3vPrmog680pnMSzf9LC7J6kXfs6lkoKfBh9VnlxusCrw2yg0qI1fHAZBLPx7mW6+me71QZsS6sVz8v8KXyrXsKTdnF50FjzHcK9HXDBtSJS5wA3fkcRYymJe0o6WMWNdgSRVpoSiWaHHmFgdMUJaYoCfhXzyl7LtNb3Q+Sveg+tJK7JaRXBLMUllOlJ6ll5Hod root@localhost"
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
build_image container rhel-edge-container

# Download the image
greenprint "📥 Downloading the container image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

# Clear container running env
greenprint "🧹 Clearing container running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

# Deal with rhel-edge container
greenprint "🗜 Extracting and running the image"
IMAGE_FILENAME="${COMPOSE_ID}-rhel84-container.tar"
sudo podman pull "oci-archive:${IMAGE_FILENAME}"
# Workaound for issue https://bugzilla.redhat.com/show_bug.cgi?id=1933774
sudo restorecon -R /var/lib/containers/storage/overlay/
sudo podman images
# Clear image file
sudo rm -f "$IMAGE_FILENAME"
# Get image id to run image
EDGE_IMAGE_ID=$(sudo podman images --filter "dangling=true" --format "{{.ID}}")
sudo podman run -d --name rhel-edge --network host "${EDGE_IMAGE_ID}"
# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Clean compose and blueprints.
greenprint "🧽 Clean up container blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete container > /dev/null

########################################################
##
## rhel-edge installer image building from container image
##
########################################################

# Write a blueprint for installer image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "installer"
description = "A rhel-edge installer image"
version = "0.0.1"
modules = []
groups = []
EOF

greenprint "📄 installer blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve installer

# Build installer image.
# Test --url arg following by URL with tailling slash for bz#1942029
build_image installer rhel-edge-installer "${URL}/"

# Download the image
greenprint "📥 Downloading the installer image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-rhel84-boot.iso"
sudo mv "${ISO_FILENAME}" /var/lib/libvirt/images

# Clean compose and blueprints.
greenprint "🧹 Clean up installer blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete installer > /dev/null

########################################################
##
## install rhel-edge image with installer(ISO)
##
########################################################

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create qcow2 file for virt install.
greenprint "🖥 Create qcow2 file for virt install"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

# Write kickstart file for ostree image installation.
greenprint "📑 Generate kickstart file"
tee "$KS_FILE" > /dev/null << STOPHERE
text
lang en_US.UTF-8
keyboard us
timezone --utc Etc/UTC

selinux --enforcing

bootloader --timeout=1 --append="net.ifnames=0 modprobe.blacklist=vc4"

network --bootproto=dhcp --device=link --activate --onboot=on

zerombr
clearpart --all --initlabel --disklabel=msdos
autopart --nohome --noswap --type=plain
ostreesetup --nogpg --osname=rhel-edge --remote=rhel-edge --url=file:///ostree/repo --ref=${OSTREE_REF}
poweroff

%post --log=/var/log/anaconda/post-install.log --erroronfail

# no sudo password for user admin
echo -e 'admin\tALL=(ALL)\tNOPASSWD: ALL' >> /etc/sudoers

# Remove any persistent NIC rules generated by udev
rm -vf /etc/udev/rules.d/*persistent-net*.rules
# And ensure that we will do DHCP on eth0 on startup
cat > /etc/sysconfig/network-scripts/ifcfg-eth0 << EOF
DEVICE="eth0"
BOOTPROTO="dhcp"
ONBOOT="yes"
TYPE="Ethernet"
PERSISTENT_DHCLIENT="yes"
EOF

echo "Zeroing out empty space."
# This forces the filesystem to reclaim space from deleted files
dd bs=1M if=/dev/zero of=/var/tmp/zeros || :
rm -f /var/tmp/zeros
echo "(Don't worry -- that out-of-space error was expected.)"

%end
STOPHERE

# Install ostree image via anaconda.
greenprint "💿 Install ostree image via installer(ISO)"
sudo virt-install  --initrd-inject="${KS_FILE}" \
                   --extra-args="inst.ks=file:/ks.cfg console=ttyS0,115200" \
                   --name="${IMAGE_KEY}"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 3072 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-type linux \
                   --os-variant ${OS_VARIANT} \
                   --location "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "📟 Start VM"
sudo virsh start "${IMAGE_KEY}"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

##################################################
##
## upgrade rhel-edge with new upgrade container
##
##################################################

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "upgrade"
description = "An upgrade rhel-edge container image"
version = "0.0.2"
modules = []
groups = []

[[packages]]
name = "python36"
version = "*"

[[packages]]
name = "wget"
version = "*"

[customizations.kernel]
name = "kernel-rt"

[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC61wMCjOSHwbVb4VfVyl5sn497qW4PsdQ7Ty7aD6wDNZ/QjjULkDV/yW5WjDlDQ7UqFH0Sr7vywjqDizUAqK7zM5FsUKsUXWHWwg/ehKg8j9xKcMv11AkFoUoujtfAujnKODkk58XSA9whPr7qcw3vPrmog680pnMSzf9LC7J6kXfs6lkoKfBh9VnlxusCrw2yg0qI1fHAZBLPx7mW6+me71QZsS6sVz8v8KXyrXsKTdnF50FjzHcK9HXDBtSJS5wA3fkcRYymJe0o6WMWNdgSRVpoSiWaHHmFgdMUJaYoCfhXzyl7LtNb3Q+Sveg+tJK7JaRXBLMUllOlJ6ll5Hod root@localhost"
home = "/home/admin/"
groups = ["wheel"]
EOF

greenprint "📄 upgrade blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing upgrade blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve upgrade

# Build upgrade image.
# Test --url arg following by URL without tailling slash for bz#1942029
build_image upgrade rhel-edge-container "$URL"

# Download the image
greenprint "📥 Downloading the upgrade image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

# Clear container running env
greenprint "🧹 Clearing container running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

# Deal with rhel-edge container
greenprint "🗜 Extracting and running the image"
IMAGE_FILENAME="${COMPOSE_ID}-rhel84-container.tar"
sudo podman pull "oci-archive:${IMAGE_FILENAME}"
# Workaound for issue https://bugzilla.redhat.com/show_bug.cgi?id=1933774
sudo restorecon -R /var/lib/containers/storage/overlay/
sudo podman images
# Clear image file
sudo rm -f "$IMAGE_FILENAME"
# Get image id to run image
EDGE_IMAGE_ID=$(sudo podman images --filter "dangling=true" --format "{{.ID}}")
sudo podman run -d --name rhel-edge --network host "${EDGE_IMAGE_ID}"
# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Get ostree commit value.
greenprint "🕹 Get ostree upgrade commit value"
UPGRADE_HASH=$(curl ${URL}/refs/heads/"${OSTREE_REF}")

# Clean compose and blueprints.
greenprint "🧽 Clean up upgrade blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete upgrade > /dev/null

# Config remote repository for upgrade
greenprint "🔗 Config remote repository for upgrade"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${GUEST_ADDRESS} 'sudo ostree remote delete rhel-edge'
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${GUEST_ADDRESS} "sudo ostree remote add --no-gpg-verify --no-sign-verify rhel-edge ${URL}"

# Upgrade image/commit.
greenprint "🗳 Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${GUEST_ADDRESS} 'sudo rpm-ostree upgrade'
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${GUEST_ADDRESS} 'nohup sudo systemctl reboot &>/dev/null & exit'

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $GUEST_ADDRESS)"
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
${GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=admin
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=rhel-edge -e ostree_commit="${UPGRADE_HASH}" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0
