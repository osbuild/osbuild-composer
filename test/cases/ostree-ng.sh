#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Install openshift client
greenprint "🔧 Installing oenshift client(oc)"
curl https://osbuild-storage.s3.amazonaws.com/oc-4.9.0-linux.tar.gz | sudo tar -xz -C /usr/local/bin/

# Start libvirtd and test it.
greenprint "🚀 Starting libvirt daemon"
sudo systemctl start libvirtd
sudo virsh list --all > /dev/null

# Install and start firewalld
greenprint "🔧 Install and start firewalld"
sudo dnf install -y firewalld
sudo systemctl enable --now firewalld

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
IMAGE_KEY="osbuild-composer-ostree-test-${TEST_UUID}"
BIOS_GUEST_ADDRESS=192.168.100.50
UEFI_GUEST_ADDRESS=192.168.100.51
PROD_REPO_URL=http://192.168.100.1/repo
PROD_REPO=/var/www/html/repo
STAGE_REPO_ADDRESS=192.168.200.1
STAGE_REPO_URL="http://${STAGE_REPO_ADDRESS}:8080/repo/"
QUAY_REPO_URL="quay.io/osbuild/testing-rhel-edge-push"
QUAY_REPO_TAG=$(tr -dc a-z0-9 < /dev/urandom | head -c 4 ; echo '')
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
# For CS8, CS9, RHEL 8.5 and above
CONTAINER_TYPE=edge-container
CONTAINER_FILENAME=container.tar
INSTALLER_TYPE=edge-installer
INSTALLER_FILENAME=installer.iso
ANSIBLE_USER_FOR_BIOS="installeruser"
OSTREE_OSNAME=rhel-edge
BOOT_ARGS="uefi"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
QUAY_CONFIG=${TEMPDIR}/quay_config.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json
FEDORA_IMAGE_DIGEST="sha256:4d76a7480ce1861c95975945633dc9d03807ffb45c64b664ef22e673798d414b"
FEDORA_LOCAL_NAME="localhost/fedora-minimal:v1"

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)

# kernel-rt package name (differs in CS8)
KERNEL_RT_PKG="kernel-rt"

# Set up variables.
SYSROOT_RO="false"

# Set FIPS variable default
FIPS="${FIPS:-false}"

# Generate the user's password hash
EDGE_USER_PASSWORD="${EDGE_USER_PASSWORD:-foobar}"
EDGE_USER_PASSWORD_SHA512=$(openssl passwd -6 -stdin <<< "${EDGE_USER_PASSWORD}")

case "${ID}-${VERSION_ID}" in
    fedora-*)
        CONTAINER_TYPE=iot-container
        INSTALLER_TYPE=iot-installer
        OSTREE_REF="fedora/${VERSION_ID}/${ARCH}/iot"
        OSTREE_OSNAME=fedora-iot
        OS_VARIANT="fedora-unknown"
        EMBEDED_CONTAINER="false"
        DIRS_FILES_CUSTOMIZATION="true"
        SYSROOT_RO="true"
        ;;
    "rhel-8"* )
        OSTREE_REF="test/rhel/8/${ARCH}/edge"
        OS_VARIANT="rhel8-unknown"
        EMBEDED_CONTAINER="true"
        DIRS_FILES_CUSTOMIZATION="true"
        ;;
    "rhel-9"* )
        OSTREE_REF="test/rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9-unknown"
        EMBEDED_CONTAINER="true"
        DIRS_FILES_CUSTOMIZATION="true"
        SYSROOT_RO="true"
        ;;
    "centos-8")
        OSTREE_REF="test/centos/8/${ARCH}/edge"
        OS_VARIANT="centos8"
        EMBEDED_CONTAINER="true"
        KERNEL_RT_PKG="kernel-rt-core"
        DIRS_FILES_CUSTOMIZATION="true"
        ;;
    "centos-9")
        OSTREE_REF="test/centos/9/${ARCH}/edge"
        OS_VARIANT="centos-stream9"
        EMBEDED_CONTAINER="true"
        BOOT_ARGS="uefi,firmware.feature0.name=secure-boot,firmware.feature0.enabled=no"
        DIRS_FILES_CUSTOMIZATION="true"
        SYSROOT_RO="true"
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac

isomount=$(mktemp -d --tmpdir=/var/tmp/)
kspath=$(mktemp -d --tmpdir=/var/tmp/)
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

    # When sudo-nopasswd is specified, a second kickstart file is added which
    # includes the %post section for creating sudoers drop-in files. This
    # kickstart file is called osbuild.ks and it %includes osbuild-base.ks at
    # the top, which is the main kickstart file created by osbuild. To inject
    # our extra %post section, lets modify the osbuild.ks file and start that
    # one so that the %include chain isn't disrupted.
    ksfile="${isomount}/osbuild.ks"
    echo "Modifying kickstart file ${ksfile}"

    ksbase=$(basename "${ksfile}")
    newksfile="${kspath}/${ksbase}"
    oldks=$(cat "${ksfile}")
    echo "Preparing modified kickstart file"
    cat > "${newksfile}" << EOFKS
${oldks}
%post --log=/var/log/anaconda/post-install.log --erroronfail
# delete local ostree repo and add external prod edge repo
ostree remote delete ${OSTREE_OSNAME}
ostree remote add --no-gpg-verify --no-sign-verify ${OSTREE_OSNAME} ${PROD_REPO_URL}
%end
EOFKS

    echo "Writing new ISO"
    if [ "${ID}" != "fedora" ] && nvrGreaterOrEqual "lorax" "34.9.18"; then
        sudo TMPDIR="/var/tmp/" mkksiso -c "console=ttyS0,115200" --ks "${newksfile}" "${iso}" "${newiso}"
    else
        sudo TMPDIR="/var/tmp/" mkksiso -c "console=ttyS0,115200" "${newksfile}" "${iso}" "${newiso}"
    fi

    echo "==== NEW KICKSTART FILE ===="
    cat "${newksfile}"
    echo "============================"
}

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

    # Start the compose.
    greenprint "🚀 Starting compose"
    if [ $# -eq 2 ]; then
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    fi
    if [ $# -eq 3 ]; then
        repo_url=$3
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" --url "$repo_url" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    fi
    if [ $# -eq 4 ]; then
        image_repo_url=$3
        registry_config=$4
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" "$blueprint_name" "$image_type" "$image_repo_url" "$registry_config" | tee "$COMPOSE_START"
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

    # Kill the journal monitor
    sudo pkill -P ${WORKER_JOURNAL_PID}

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        redprint "Something went wrong with the compose. 😢"
        exit 1
    fi
}

# Wait for the ssh server up to be.
# Test user admin added by edge-container bp
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
    # Remove tag from quay.io repo
    skopeo delete --creds "${V2_QUAY_USERNAME}:${V2_QUAY_PASSWORD}" "docker://${QUAY_REPO_URL}:${QUAY_REPO_TAG}"

    # Clear vm
    if [[ $(sudo virsh domstate "${IMAGE_KEY}-uefi") == "running" ]]; then
        sudo virsh destroy "${IMAGE_KEY}-uefi"
    fi
    sudo virsh undefine "${IMAGE_KEY}-uefi" --nvram
    # Remove qcow2 file.
    sudo rm -f "$LIBVIRT_UEFI_IMAGE_PATH"
    # Clear integration network
    sudo virsh net-destroy integration
    sudo virsh net-undefine integration

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
        redprint "❌ Failed"
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

# Prepare stage repo network
greenprint "🔧 Prepare stage repo network"
sudo podman network inspect edge >/dev/null 2>&1 || sudo podman network create --driver=bridge --subnet=192.168.200.0/24 --gateway=192.168.200.254 edge

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
name = "python3"
version = "*"

[[packages]]
name = "sssd"
version = "*"

[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "${EDGE_USER_PASSWORD_SHA512}"
key = "${SSH_KEY_PUB}"
home = "/home/admin/"
groups = ["wheel"]
EOF

# RHEL 8.7 and 9.1 later support embeded container in commit
if [[ "${EMBEDED_CONTAINER}" == "true" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[containers]]
source = "quay.io/fedora/fedora:latest"

[[containers]]
source = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal@${FEDORA_IMAGE_DIGEST}"
name = "${FEDORA_LOCAL_NAME}"
EOF
fi

# Add directory and files customization, and services customization for testing
if [[ "${DIRS_FILES_CUSTOMIZATION}" == "true" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[customizations.directories]]
path = "/etc/custom_dir/dir1"
user = 1020
group = 1020
mode = "0770"
ensure_parents = true

[[customizations.files]]
path = "/etc/systemd/system/custom.service"
data = "[Unit]\nDescription=Custom service\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/usr/bin/false\n[Install]\nWantedBy=multi-user.target\n"

[[customizations.files]]
path = "/etc/custom_file.txt"
data = "image builder is the best\n"

[[customizations.directories]]
path = "/etc/systemd/system/custom.service.d"

[[customizations.files]]
path = "/etc/systemd/system/custom.service.d/override.conf"
data = "[Service]\nExecStart=\nExecStart=/usr/bin/cat /etc/custom_file.txt\n"

[customizations.services]
enabled = ["custom.service"]
EOF
fi

greenprint "📄 container blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing container blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve container

# Write the registry configuration.
greenprint "📄 Perparing quay.io config to push image"
tee "$QUAY_CONFIG" > /dev/null << EOF
provider = "container"
[settings]
username = "$V2_QUAY_USERNAME"
password = "$V2_QUAY_PASSWORD"
EOF

# Build container image.
build_image container "$CONTAINER_TYPE" "${QUAY_REPO_URL}:${QUAY_REPO_TAG}" "$QUAY_CONFIG"

# Run edge stage repo
greenprint "🛰 Running edge stage repo"
sudo podman pull --creds "${V2_QUAY_USERNAME}:${V2_QUAY_PASSWORD}" "docker://${QUAY_REPO_URL}:${QUAY_REPO_TAG}"
sudo podman run -d --name rhel-edge --network edge --ip "$STAGE_REPO_ADDRESS" "${QUAY_REPO_URL}:${QUAY_REPO_TAG}"

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

if [ "${FIPS}" == "true" ]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[customizations]
fips = ${FIPS}
EOF
fi

tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[customizations.user]]
name = "installeruser"
description = "Added by installer blueprint"
password = "${EDGE_USER_PASSWORD_SHA512}"
key = "${SSH_KEY_PUB}"
home = "/home/installeruser/"
groups = ["wheel"]

[customizations.installer]
unattended = true
sudo-nopasswd = ["admin", "installeruser"]
EOF

greenprint "📄 installer blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve installer

# Build installer image.
# Test --url arg following by URL with tailling slash for bz#1942029
build_image installer "${INSTALLER_TYPE}" "${PROD_REPO_URL}/"

# Download the image
greenprint "📥 Downloading the installer image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
modksiso "${ISO_FILENAME}" "/var/lib/libvirt/images/${ISO_FILENAME}"
sudo rm "${ISO_FILENAME}"

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
LIBVIRT_BIOS_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-bios.qcow2
LIBVIRT_UEFI_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-uefi.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_BIOS_IMAGE_PATH}" 20G
sudo qemu-img create -f qcow2 "${LIBVIRT_UEFI_IMAGE_PATH}" 20G

##################################################
##
## Install and test Edge image on BIOS VM
##
##################################################
# Install ostree image via anaconda.
greenprint "💿 Install ostree image via installer(ISO) on BIOS VM"
sudo virt-install  --name="${IMAGE_KEY}-bios" \
                   --disk path="${LIBVIRT_BIOS_IMAGE_PATH}",format=qcow2 \
                   --ram 2048 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "📟 Start BIOS VM"
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

# With new ostree-libs-2022.6-3, edge vm needs to reboot twice to make the /sysroot readonly
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "admin@${BIOS_GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
# Sleep 10 seconds here to make sure vm restarted already
sleep 10
# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $BIOS_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

# Get ostree commit value.
greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Run Edge test on BIOS VM
# Add instance IP address into /etc/ansible/hosts
# Run BIOS VM test with installeruser added by edge-installer bp as ansible user
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${BIOS_GUEST_ADDRESS}
[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${ANSIBLE_USER_FOR_BIOS}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
greenprint "📼 Run Edge tests on BIOS VM"
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type="$OSTREE_OSNAME" \
    -e ostree_commit="${INSTALL_HASH}" \
    -e skip_rollback_test="true" \
    -e embeded_container="${EMBEDED_CONTAINER}" \
    -e test_custom_dirs_files="${DIRS_FILES_CUSTOMIZATION}" \
    -e sysroot_ro="$SYSROOT_RO" \
    -e fips="${FIPS}" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Clean up BIOS VM
greenprint "🧹 Clean up BIOS VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}-bios") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-bios"
fi
sudo virsh undefine "${IMAGE_KEY}-bios"
sudo rm -f "$LIBVIRT_BIOS_IMAGE_PATH"

##################################################
##
## Install, upgrade and test Edge image on UEFI VM
##
##################################################

# Install ostree image via anaconda.
greenprint "💿 Install ostree image via installer(ISO) on UEFI VM"
sudo virt-install  --name="${IMAGE_KEY}-uefi"\
                   --disk path="${LIBVIRT_UEFI_IMAGE_PATH}",format=qcow2 \
                   --ram 2048 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:31 \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --boot "$BOOT_ARGS" \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "💻 Start UEFI VM"
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

# With new ostree-libs-2022.6-3, edge vm needs to reboot twice to make the /sysroot readonly
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "admin@${UEFI_GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
# Sleep 10 seconds here to make sure vm restarted already
sleep 10
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

# Get ostree commit value.
greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Run Edge test on UEFI VM
# Add instance IP address into /etc/ansible/hosts
# Run UEFI VM test with installeruser added by edge-installer bp as ansible user
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${UEFI_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${ANSIBLE_USER_FOR_BIOS}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
greenprint "📼 Run Edge tests on UEFI VM"
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type="$OSTREE_OSNAME" \
    -e ostree_commit="${INSTALL_HASH}" \
    -e skip_rollback_test="true" \
    -e embeded_container="${EMBEDED_CONTAINER}" \
    -e test_custom_dirs_files="${DIRS_FILES_CUSTOMIZATION}" \
    -e sysroot_ro="$SYSROOT_RO" \
    -e fips="${FIPS}" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0

# Check image installation result
check_result

##################################################
##
## upgrade rhel-edge with new upgrade commit
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
name = "python3"
version = "*"

[[packages]]
name = "sssd"
version = "*"

[[packages]]
name = "wget"
version = "*"
EOF


# No RT kernel in Fedora
if [[ "$ID" != "fedora" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[customizations.kernel]
name = "${KERNEL_RT_PKG}"
EOF
fi

# RHEL 8.7 and 9.1 later support embeded container in commit
if [[ "${EMBEDED_CONTAINER}" == "true" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[containers]]
source = "quay.io/fedora/fedora:latest"

[[containers]]
source = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal@${FEDORA_IMAGE_DIGEST}"
name = "${FEDORA_LOCAL_NAME}"
EOF
fi

# Add directory and files customization, and services customization for testing
if [[ "${DIRS_FILES_CUSTOMIZATION}" == "true" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[customizations.directories]]
path = "/etc/custom_dir/dir1"
user = 1020
group = 1020
mode = "0770"
ensure_parents = true

[[customizations.files]]
path = "/etc/systemd/system/custom.service"
data = "[Unit]\nDescription=Custom service\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/usr/bin/false\n[Install]\nWantedBy=multi-user.target\n"

[[customizations.files]]
path = "/etc/custom_file.txt"
data = "image builder is the best\n"

[[customizations.directories]]
path = "/etc/systemd/system/custom.service.d"

[[customizations.files]]
path = "/etc/systemd/system/custom.service.d/override.conf"
data = "[Service]\nExecStart=\nExecStart=/usr/bin/cat /etc/custom_file.txt\n"

[customizations.services]
enabled = ["custom.service"]
EOF
fi

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

# Upgrade image/commit.
# Test user admin added by edge-container bp
greenprint "🗳 Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${UEFI_GUEST_ADDRESS} "sudo rpm-ostree upgrade --os=${OSTREE_OSNAME}"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@${UEFI_GUEST_ADDRESS} 'nohup sudo systemctl reboot &>/dev/null & exit'

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
# Test user installeruser added by edge-installer bp
# User installer still exists after upgrade but upgrade bp does not contain installeruer
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${UEFI_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${ANSIBLE_USER_FOR_BIOS}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type="$OSTREE_OSNAME" \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e skip_rollback_test="true" \
    -e embeded_container="${EMBEDED_CONTAINER}" \
    -e test_custom_dirs_files="${DIRS_FILES_CUSTOMIZATION}" \
    -e sysroot_ro="$SYSROOT_RO" \
    -e fips="${FIPS}" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0
