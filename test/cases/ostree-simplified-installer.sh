#!/bin/bash
set -euo pipefail

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Start firewalld
sudo systemctl enable --now firewalld
sudo pip3 install yq

# Start libvirtd and test it.
greenprint "🚀 Starting libvirt daemon"
sudo systemctl start libvirtd
sudo virsh list --all > /dev/null

# Set a customized dnsmasq configuration for libvirt so we always get the
# same address on bootup.
sudo tee /tmp/integration.xml > /dev/null << EOF
<network xmlns:dnsmasq='http://libvirt.org/schemas/network/dnsmasq/1.0'>
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
      <host mac='34:49:22:B0:83:30' name='vm-httpboot' ip='192.168.100.50'/>
      <host mac='34:49:22:B0:83:31' name='vm-uefi-01' ip='192.168.100.51'/>
      <host mac='34:49:22:B0:83:32' name='vm-uefi-02' ip='192.168.100.52'/>
    </dhcp>
  </ip>
  <dnsmasq:options>
    <dnsmasq:option value='dhcp-vendorclass=set:efi-http,HTTPClient:Arch:00016'/>
    <dnsmasq:option value='dhcp-option-force=tag:efi-http,60,HTTPClient'/>
    <dnsmasq:option value='dhcp-boot=tag:efi-http,&quot;http://192.168.100.1/httpboot/EFI/BOOT/BOOTX64.EFI&quot;'/>
  </dnsmasq:options>
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
HTTP_GUEST_ADDRESS=192.168.100.50
PUB_KEY_GUEST_ADDRESS=192.168.100.51
ROOT_CERT_GUEST_ADDRESS=192.168.100.52
PROD_REPO_URL=http://192.168.100.1/repo
PROD_REPO=/var/www/html/repo
STAGE_REPO_ADDRESS=192.168.200.1
STAGE_REPO_URL="http://${STAGE_REPO_ADDRESS}:8080/repo/"
FDO_SERVER_ADDRESS=192.168.200.2
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
CONTAINER_TYPE=edge-container
CONTAINER_FILENAME=container.tar
INSTALLER_TYPE=edge-simplified-installer
INSTALLER_FILENAME=simplified-installer.iso
MEMORY=3072
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

case "${ID}-${VERSION_ID}" in
    "rhel-8.7")
        OSTREE_REF="rhel/8/${ARCH}/edge"
        OS_VARIANT="rhel8-unknown"
        ;;
    "rhel-9.1")
        OSTREE_REF="rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9-unknown"
        ;;
    "centos-8")
        OSTREE_REF="centos/8/${ARCH}/edge"
        OS_VARIANT="centos8"
        ;;
    "centos-9")
        OSTREE_REF="centos/9/${ARCH}/edge"
        OS_VARIANT="centos-stream9"
        BOOT_ARGS="uefi,firmware.feature0.name=secure-boot,firmware.feature0.enabled=no"
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

    # Clean up BIOS VM
    greenprint "🧹 Clean up BIOS VM"
    if [[ $(sudo virsh domstate "${IMAGE_KEY}-simplified_iso_without_fdo") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}-simplified_iso_without_fdo"
    fi
    sudo virsh undefine "${IMAGE_KEY}-simplified_iso_without_fdo" --nvram
    sudo virsh vol-delete --pool images "$LIBVIRT_IMAGE_PATH"

    # Remove any status containers if exist
    sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
    # Remove all images
    sudo podman rmi -f -a

    # Remove prod repo
    sudo rm -rf "$PROD_REPO"

    # Remomve tmp dir.
    sudo rm -rf "$TEMPDIR"

    # Stop prod repo http service
    sudo systemctl disable --now httpd
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

###########################################################
##
## Prepare edge prod and stage repo
##
###########################################################
greenprint "🔧 Prepare edge prod repo"
# Start prod repo web service
# osbuild-composer-tests have mod_ssl as a dependency. The package installs
# an example configuration which automatically enabled httpd on port 443, but
# that one is already in use. Remove the default configuration as it is useless
# anyway.
sudo rm -f /etc/httpd/conf.d/ssl.conf
sudo systemctl enable --now httpd.service

# Have a clean prod repo
sudo rm -rf "$PROD_REPO"
sudo mkdir -p "$PROD_REPO"
sudo ostree --repo="$PROD_REPO" init --mode=archive
sudo ostree --repo="$PROD_REPO" remote add --no-gpg-verify edge-stage "$STAGE_REPO_URL"

# Clear container running env
greenprint "🧹 Clearing container running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

# Prepare stage repo network, also needed for FDO AIO to correctly resolve ips
greenprint "🔧 Prepare stage repo network"
sudo podman network inspect edge >/dev/null 2>&1 || sudo podman network create --driver=bridge --subnet=192.168.200.0/24 --gateway=192.168.200.254 edge

###########################################################
##
## Prepare fdo AIO server
##
###########################################################
greenprint "🔧 Prepare fdo AIO server"

greenprint "🔧 Prepare fdo AIO configuration"
sudo mkdir aio
sudo podman run -v "$PWD"/aio/:/aio:z \
  "quay.io/fido-fdo/aio:nightly" \
  aio --directory aio generate-configs-and-keys --contact-hostname "$FDO_SERVER_ADDRESS"

# TODO: tweak config aio/configs/serviceinfo_api_server.yml to test basic FDO functionalities
#       like adding user/key/pwd, re-encryption, files, commands etc etc
# TODO: check tpm2 config
sudo /usr/local/bin/yq -iy '.service_info.diskencryption_clevis |= [{disk_label: "/dev/vda4", reencrypt: true, binding: {pin: "tpm2", config: "{}"}}]' aio/configs/serviceinfo_api_server.yml

greenprint "🔧 Prepare fdo AIO manufacturing DIUN"
DIUN_PUB_KEY_ROOT_CERTS=$(sudo cat aio/keys/diun_cert.pem)
# shellcheck disable=SC2116
DIUN_PUB_KEY_HASH=$(echo "sha256:$(sudo openssl x509 -fingerprint -sha256 -noout -in aio/keys/diun_cert.pem | cut -d"=" -f2 | sed 's/://g')")

greenprint "🔧 Starting fdo AIO server"
sudo podman run -d \
  --ip "$FDO_SERVER_ADDRESS" \
  --name fdo-aio \
  --network edge \
  -v "$PWD"/aio/:/aio:z \
  "quay.io/fido-fdo/aio:nightly" \
  aio --directory aio

# Wait for fdo server to be running
until [ "$(curl -X POST http://${FDO_SERVER_ADDRESS}:8080/ping)" == "pong" ]; do
    sleep 1;
done;

###############################
##
## Build edge-container image
##
###############################

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

[[packages]]
name = "sssd"
version = "*"

[customizations.kernel]
name = "kernel-rt"

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

# Deal with stage repo image
greenprint "🗜 Starting container"
IMAGE_FILENAME="${COMPOSE_ID}-${CONTAINER_FILENAME}"
sudo podman pull "oci-archive:${IMAGE_FILENAME}"
sudo podman images
# Run edge stage repo
greenprint "🛰 Running edge stage repo"
# Get image id to run image
EDGE_IMAGE_ID=$(sudo podman images --filter "dangling=true" --format "{{.ID}}")
sudo podman run -d --name rhel-edge --network edge --ip "$STAGE_REPO_ADDRESS" "$EDGE_IMAGE_ID"
# Clear image file
sudo rm -f "$IMAGE_FILENAME"

# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Sync installer edge content
greenprint "📡 Sync installer content from stage repo"
sudo ostree --repo="$PROD_REPO" pull --mirror edge-stage "$OSTREE_REF"

# Clean compose and blueprints.
greenprint "🧽 Clean up container blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete container > /dev/null

########################################################################
##
## Build edge-simplified-installer with diun_pub_key_insecure enabled
##
########################################################################
# Write a blueprint for installer image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "installer"
description = "A rhel-edge simplified-installer image"
version = "0.0.1"
modules = []
groups = []

[customizations]
installation_device = "/dev/vda"

[customizations.fdo]
manufacturing_server_url="http://${FDO_SERVER_ADDRESS}:8080"
diun_pub_key_insecure="true"
EOF

greenprint "📄 installer blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve installer

# Build installer image.
build_image installer "${INSTALLER_TYPE}" "${PROD_REPO_URL}"

# Download the image
greenprint "📥 Downloading the installer image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
sudo cp "${ISO_FILENAME}" /var/lib/libvirt/images

# Clean compose and blueprints.
greenprint "🧹 Clean up installer blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete installer > /dev/null

HTTPD_PATH="/var/www/html"
GRUB_CFG=${HTTPD_PATH}/httpboot/EFI/BOOT/grub.cfg

greenprint "📋 Mount simplified installer iso and copy content to webserver/httpboot"
sudo mkdir -p ${HTTPD_PATH}/httpboot
sudo mkdir /mnt/installer
sudo mount -o loop "${ISO_FILENAME}" /mnt/installer
sudo cp -R /mnt/installer/* ${HTTPD_PATH}/httpboot/
sudo chmod -R +r ${HTTPD_PATH}/httpboot/*

greenprint "📋 Update grub.cfg file for http boot"
sudo sed -i 's/timeout=60/timeout=10/' "${GRUB_CFG}"
sudo sed -i 's/coreos.inst.install_dev=\/dev\/sda/coreos.inst.install_dev=\/dev\/vda/' "${GRUB_CFG}"
sudo sed -i 's/linux \/images\/pxeboot\/vmlinuz/linuxefi \/httpboot\/images\/pxeboot\/vmlinuz/' "${GRUB_CFG}"
sudo sed -i 's/initrd \/images\/pxeboot\/initrd.img/initrdefi \/httpboot\/images\/pxeboot\/initrd.img/' "${GRUB_CFG}"
sudo sed -i 's/coreos.inst.image_file=\/run\/media\/iso\/image.raw.xz/coreos.inst.image_url=http:\/\/192.168.100.1\/httpboot\/image.raw.xz/' "${GRUB_CFG}"

greenprint "📋 Create libvirt image disk"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

greenprint "checking running containers"
sudo podman ps -a

greenprint "📋 Install edge vm via http boot"
sudo virt-install --name="${IMAGE_KEY}-http"\
                  --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                  --ram "${MEMORY}" \
                  --vcpus 2 \
                  --network network=integration,mac=34:49:22:B0:83:30 \
                  --os-type linux \
                  --os-variant "${OS_VARIANT}" \
                  --pxe \
                  --boot "$BOOT_ARGS" \
                  --tpm backend.type=emulator,backend.version=2.0,model=tpm-crb \
                  --nographics \
                  --noautoconsole \
                  --wait=15 \
                  --noreboot

# Installation can get stuck, destroying VM helps
# See https://github.com/osbuild/osbuild-composer/issues/2413
if [[ $(sudo virsh domstate "${IMAGE_KEY}-http") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-http"
fi

# Start VM.
greenprint "💻 Start HTTP BOOT VM"
sudo virsh start "${IMAGE_KEY}-http"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $HTTP_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${HTTP_GUEST_ADDRESS}

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
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=redhat -e ostree_commit="${INSTALL_HASH}" -e edge_type=edge-simplified-installer -e fdo_credential="true" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Clean up BIOS VM
greenprint "🧹 Clean up BIOS VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}-http") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-http"
fi
sudo virsh undefine "${IMAGE_KEY}-http" --nvram
sudo virsh vol-delete --pool images "$LIBVIRT_IMAGE_PATH"
sudo umount /mnt/installer
sudo rm -rf /mnt/installer "${ISO_FILENAME}"
sudo rm -rf "/var/lib/libvirt/images/${ISO_FILENAME}"

####################################################################
##
## Build edge-simplified-installer with diun_pub_key_hash enabled
##
####################################################################

tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "fdosshkey"
description = "A rhel-edge simplified-installer image"
version = "0.0.1"
modules = []
groups = []

[customizations]
installation_device = "/dev/vda"

[customizations.fdo]
manufacturing_server_url="http://${FDO_SERVER_ADDRESS}:8080"
diun_pub_key_hash="${DIUN_PUB_KEY_HASH}"
EOF

greenprint "📄 fdosshkey blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing fdosshkey blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve fdosshkey

# Build fdosshkey image.
build_image fdosshkey "${INSTALLER_TYPE}" "${PROD_REPO_URL}"

# Download the image
greenprint "📥 Downloading the fdosshkey image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
sudo cp "${ISO_FILENAME}" /var/lib/libvirt/images

# Clean compose and blueprints.
greenprint "🧹 Clean up fdosshkey blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete fdosshkey > /dev/null

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create qcow2 file for virt install.
greenprint "🖥 Create qcow2 file for virt install"
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

greenprint "💿 Install ostree image via installer(ISO) on UEFI VM"
sudo virt-install  --name="${IMAGE_KEY}-fdosshkey"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram "${MEMORY}" \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:31 \
                   --os-type linux \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --boot "$BOOT_ARGS" \
                   --tpm backend.type=emulator,backend.version=2.0,model=tpm-crb \
                   --nographics \
                   --noautoconsole \
                   --wait=15 \
                   --noreboot

# Installation can get stuck, destroying VM helps
# See https://github.com/osbuild/osbuild-composer/issues/2413
if [[ $(sudo virsh domstate "${IMAGE_KEY}-fdosshkey") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-fdosshkey"
fi

# Start VM.
greenprint "💻 Start UEFI VM"
sudo virsh start "${IMAGE_KEY}-fdosshkey"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $PUB_KEY_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${PUB_KEY_GUEST_ADDRESS}

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
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=redhat -e ostree_commit="${INSTALL_HASH}" -e edge_type=edge-simplified-installer -e fdo_credential="true" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Clean up BIOS VM
greenprint "🧹 Clean up VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}-fdosshkey") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-fdosshkey"
fi
sudo virsh undefine "${IMAGE_KEY}-fdosshkey" --nvram
sudo virsh vol-delete --pool images "$LIBVIRT_IMAGE_PATH"

##################################################################
##
## Build edge-simplified-installer with diun_pub_key_root_certs
##
##################################################################

tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "fdorootcert"
description = "A rhel-edge simplified-installer image"
version = "0.0.1"
modules = []
groups = []

[customizations]
installation_device = "/dev/vda"

[customizations.fdo]
manufacturing_server_url="http://${FDO_SERVER_ADDRESS}:8080"
diun_pub_key_root_certs="""
${DIUN_PUB_KEY_ROOT_CERTS}"""
EOF

greenprint "📄 fdosshkey blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve fdorootcert

# Build fdorootcert image.
build_image fdorootcert "${INSTALLER_TYPE}" "${PROD_REPO_URL}/"

# Download the image
greenprint "📥 Downloading the fdorootcert image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
sudo cp "${ISO_FILENAME}" /var/lib/libvirt/images

# Clean compose and blueprints.
greenprint "🧹 Clean up fdorootcert blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete fdorootcert > /dev/null

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create qcow2 file for virt install.
greenprint "🖥 Create qcow2 file for virt install"
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

greenprint "💿 Install ostree image via installer(ISO) on UEFI VM"
sudo virt-install  --name="${IMAGE_KEY}-fdorootcert"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram "${MEMORY}" \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:32 \
                   --os-type linux \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --boot "$BOOT_ARGS" \
                   --tpm backend.type=emulator,backend.version=2.0,model=tpm-crb \
                   --nographics \
                   --noautoconsole \
                   --wait=15 \
                   --noreboot

# Installation can get stuck, destroying VM helps
# See https://github.com/osbuild/osbuild-composer/issues/2413
if [[ $(sudo virsh domstate "${IMAGE_KEY}-fdorootcert") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-fdorootcert"
fi

# Start VM.
greenprint "💻 Start UEFI VM"
sudo virsh start "${IMAGE_KEY}-fdorootcert"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $ROOT_CERT_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${ROOT_CERT_GUEST_ADDRESS}

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
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=redhat -e ostree_commit="${INSTALL_HASH}" -e edge_type=edge-simplified-installer -e fdo_credential="true" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

greenprint "🧹 Clean up VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}-fdorootcert") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-fdorootcert"
fi
sudo virsh undefine "${IMAGE_KEY}-fdorootcert" --nvram
sudo virsh vol-delete --pool images "$LIBVIRT_IMAGE_PATH"

##################################################################
##
## Build edge-simplified-installer without FDO 
##
##################################################################

tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "simplified_iso_without_fdo"
description = "A rhel-edge simplified-installer image without FDO"
version = "0.0.1"
modules = []
groups = []

[customizations]
installation_device = "/dev/vda"

EOF

greenprint "📄 simplified_iso_without_fdo blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve simplified_iso_without_fdo

# Build simplified installer iso image.
build_image simplified_iso_without_fdo "${INSTALLER_TYPE}" "${PROD_REPO_URL}/"

# Download the image
greenprint "📥 Downloading the simplified_iso_without_fdo image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
sudo cp "${ISO_FILENAME}" /var/lib/libvirt/images

# Clean compose and blueprints.
greenprint "🧹 Clean up simplified_iso_without_fdo blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete simplified_iso_without_fdo > /dev/null

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create qcow2 file for virt install.
greenprint "🖥 Create qcow2 file for virt install"
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

greenprint "💿 Install ostree image via installer(ISO) on UEFI VM"
sudo virt-install  --name="${IMAGE_KEY}-simplified_iso_without_fdo"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram "${MEMORY}" \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:32 \
                   --os-type linux \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --boot "$BOOT_ARGS" \
                   --tpm backend.type=emulator,backend.version=2.0,model=tpm-crb \
                   --nographics \
                   --noautoconsole \
                   --wait=15 \
                   --noreboot

# Installation can get stuck, destroying VM helps
# See https://github.com/osbuild/osbuild-composer/issues/2413
if [[ $(sudo virsh domstate "${IMAGE_KEY}-simplified_iso_without_fdo") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-simplified_iso_without_fdo"
fi

# Start VM.
greenprint "💻 Start UEFI VM"
sudo virsh start "${IMAGE_KEY}-simplified_iso_without_fdo"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $ROOT_CERT_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${ROOT_CERT_GUEST_ADDRESS}

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
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=redhat -e ostree_commit="${INSTALL_HASH}" -e edge_type=edge-simplified-installer -e fdo_credential="false" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

########################
##
## Build upgrade image
##
########################

# Write a blueprint for ostree image.
# NB: no ssh key in this blueprint for the admin user
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
name = "sssd"
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
build_image upgrade  "${CONTAINER_TYPE}" "$PROD_REPO_URL"

# Download the image
greenprint "📥 Downloading the upgrade image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

# Delete installation rhel-edge container and its image
greenprint "🧹 Delete installation rhel-edge container and its image"
# Remove rhel-edge container if exists
sudo podman ps -q --filter name=rhel-edge --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove container image if exists
sudo podman images --filter "dangling=true" --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rmi -f

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
sudo podman run -d --name rhel-edge --network edge --ip "$STAGE_REPO_ADDRESS" "$EDGE_IMAGE_ID"
# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Pull upgrade to prod mirror
greenprint "⛓ Pull upgrade to prod mirror"
sudo ostree --repo="$PROD_REPO" pull --mirror edge-stage "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO" static-delta generate "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO" summary -u

# Get ostree commit value.
greenprint "🕹 Get ostree upgrade commit value"
UPGRADE_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Clean compose and blueprints.
greenprint "🧽 Clean up upgrade blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete upgrade > /dev/null

greenprint "🗳 Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${ROOT_CERT_GUEST_ADDRESS} "echo ${EDGE_USER_PASSWORD} |sudo -S rpm-ostree upgrade"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${ROOT_CERT_GUEST_ADDRESS} "echo ${EDGE_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $ROOT_CERT_GUEST_ADDRESS)"
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
${ROOT_CERT_GUEST_ADDRESS}

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
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory -e image_type=redhat -e ostree_commit="${UPGRADE_HASH}" -e edge_type=edge-simplified-installer -e fdo_credential="false" /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0
