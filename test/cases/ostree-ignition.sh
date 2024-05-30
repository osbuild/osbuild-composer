#!/bin/bash
set -euox pipefail

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

function cleanup_on_exit() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"
}
trap cleanup_on_exit EXIT

# Install and start firewalld
greenprint "🔧 Install and start firewalld"
sudo dnf install -y firewalld
sudo systemctl enable --now firewalld

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
      <host mac='34:49:22:B0:83:30' name='vm-01' ip='192.168.100.50'/>
      <host mac='34:49:22:B0:83:31' name='vm-02' ip='192.168.100.51'/>
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
IMAGE_KEY="ostree-installer-${TEST_UUID}"
SIMPLIFIED_GUEST_ADDRESS=192.168.100.50
RAW_GUEST_ADDRESS=192.168.100.51
# PROD_REPO_1 is for simplified installer test
# PROD_REPO_2 is for raw image test
PROD_REPO_1_URL=http://192.168.100.1/repo1
PROD_REPO_1=/var/www/html/repo1
PROD_REPO_2_URL=http://192.168.100.1/repo2
PROD_REPO_2=/var/www/html/repo2
STAGE_REPO_ADDRESS=192.168.200.1
STAGE_REPO_URL="http://${STAGE_REPO_ADDRESS}:8080/repo/"
IGNITION_SERVER_FOLDER=/var/www/html/ignition
IGNITION_SERVER_URL=http://192.168.100.1/ignition
CONTAINER_TYPE=edge-container
CONTAINER_FILENAME=container.tar
INSTALLER_TYPE=edge-simplified-installer
INSTALLER_FILENAME=simplified-installer.iso
RAW_TYPE=edge-raw-image
RAW_FILENAME=image.raw.xz
# Workaround BZ#2108646
BOOT_ARGS="uefi"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# Setup log artifacts folder
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)

# Ignition
IGNITION_USER=core
IGNITION_USER_PASSWORD=foobar

# Mount /sysroot as RO by new ostree-libs-2022.6-3.el9.x86_64
# It's RHEL 9.2 and above, CS9, Fedora 37 and above ONLY
SYSROOT_RO="true"

case "${ID}-${VERSION_ID}" in
    "rhel-9."*)
        OSTREE_REF="rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9-unknown"
        ;;
    "centos-9")
        OSTREE_REF="centos/9/${ARCH}/edge"
        OS_VARIANT="centos-stream9"
        BOOT_ARGS="uefi,firmware.feature0.name=secure-boot,firmware.feature0.enabled=no"
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-installer-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-installer-${COMPOSE_ID}.json

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
    SSH_STATUS=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${1}" '/bin/bash -c "echo -n READY"')
    if [[ $SSH_STATUS == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

# Clean up our mess.
clean_up () {
    greenprint "🧼 Cleaning up"

    # Clear integration network
    sudo virsh net-destroy integration
    sudo virsh net-undefine integration

    # Remove any status containers if exist
    sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
    # Remove all images
    sudo podman rmi -f -a

    # Remove prod repo
    sudo rm -rf "$PROD_REPO_1"
    sudo rm -rf "$PROD_REPO_2"
    sudo rm -rf "$IGNITION_SERVER_FOLDER"

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
# Start ostree repo web service
# osbuild-composer-tests have mod_ssl as a dependency. The package installs
# an example configuration which automatically enabled httpd on port 443, but
# that one is already in use. Remove the default configuration as it is useless
# anyway.
sudo rm -f /etc/httpd/conf.d/ssl.conf
sudo systemctl enable --now httpd.service
# Have a clean prod repo for raw image test and simplified installer test
greenprint "🔧 Prepare edge prod repo for simplified installer test"
sudo rm -rf "$PROD_REPO_1"
sudo mkdir -p "$PROD_REPO_1"
sudo ostree --repo="$PROD_REPO_1" init --mode=archive
sudo ostree --repo="$PROD_REPO_1" remote add --no-gpg-verify edge-stage "$STAGE_REPO_URL"

greenprint "🔧 Prepare edge prod repo for raw image test"
sudo rm -rf "$PROD_REPO_2"
sudo mkdir -p "$PROD_REPO_2"
sudo ostree --repo="$PROD_REPO_2" init --mode=archive
sudo ostree --repo="$PROD_REPO_2" remote add --no-gpg-verify edge-stage "$STAGE_REPO_URL"

# Prepare stage repo network
greenprint "🔧 Prepare stage repo network"
sudo podman network inspect edge >/dev/null 2>&1 || sudo podman network create --driver=bridge --subnet=192.168.200.0/24 --gateway=192.168.200.254 edge

# Clear container running env
greenprint "🧹 Clearing container running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

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
sudo ostree --repo="$PROD_REPO_1" pull --mirror edge-stage "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO_2" pull --mirror edge-stage "$OSTREE_REF"

# Clean rhel-edge container
sudo podman rm -f rhel-edge
sudo podman rmi -f "$EDGE_IMAGE_ID"

# Clean compose and blueprints.
greenprint "🧽 Clean up container blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete container > /dev/null

# Generate ignition configuration
sudo mkdir -p "$IGNITION_SERVER_FOLDER"
IGNITION_CONFIG_PATH="${IGNITION_SERVER_FOLDER}/config.ign"
sudo tee "$IGNITION_CONFIG_PATH" > /dev/null << EOF
{
  "ignition": {
    "config": {
      "merge": [
        {
          "source": "${IGNITION_SERVER_URL}/sample.ign"
        }
      ]
    },
    "timeouts": {
      "httpTotal": 30
    },
    "version": "3.3.0"
  },
  "passwd": {
    "users": [
      {
        "groups": [
          "wheel"
        ],
        "name": "$IGNITION_USER",
        "passwordHash": "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl.",
        "sshAuthorizedKeys": [
          "$SSH_KEY_PUB"
        ]
      }
    ]
  }
}
EOF

# Generate enbeded ignition configuration
sudo dnf install -y butane
tee "${TEMPDIR}/config.bu" > /dev/null << EOF
variant: r4e
version: 1.0.0
ignition:
  timeouts:
    http_total: 30
  config:
    merge:
      - source: ${IGNITION_SERVER_URL}/sample.ign
passwd:
  users:
    - name: core
      groups:
        - wheel
      password_hash: "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
      ssh_authorized_keys:
        - $SSH_KEY_PUB
EOF
butane --pretty --strict "${TEMPDIR}/config.bu" > "${TEMPDIR}/config.ign"
# key "customizations.ignition.embedded.config": strings cannot contain newlines
IGNITION_B64=$(base64 -w 0 < "${TEMPDIR}/config.ign")

IGNITION_CONFIG_SAMPLE_PATH="${IGNITION_SERVER_FOLDER}/sample.ign"
sudo tee "$IGNITION_CONFIG_SAMPLE_PATH" > /dev/null << EOF
{
  "ignition": {
    "version": "3.3.0"
  },
  "storage": {
    "files": [
      {
        "path": "/usr/local/bin/startup.sh",
        "contents": {
          "compression": "",
          "source": "data:;base64,IyEvYmluL2Jhc2gKZWNobyAiSGVsbG8sIFdvcmxkISIK"
        },
        "mode": 493
      }
    ]
  },
  "systemd": {
    "units": [
      {
        "contents": "[Unit]\nDescription=A hello world unit!\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/usr/local/bin/startup.sh\n[Install]\nWantedBy=multi-user.target\n",
        "enabled": true,
        "name": "hello.service"
      },
      {
        "dropins": [
          {
            "contents": "[Service]\nEnvironment=LOG_LEVEL=trace\n",
            "name": "log_trace.conf"
          }
        ],
        "name": "fdo-client-linuxapp.service"
      }
    ]
  }
}
EOF

######################################################################
##
## Build edge-simplified-installer with embedded ignition configured
##
######################################################################
# Write a blueprint for installer image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "installer"
description = "A rhel-edge simplified-installer image"
version = "0.0.1"
modules = []
groups = []

[customizations]
installation_device = "/dev/vdb"

[customizations.ignition.embedded]
config = "$IGNITION_B64"
EOF

greenprint "📄 installer blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve installer

# Build installer image.
build_image installer "${INSTALLER_TYPE}" "${PROD_REPO_1_URL}"

# Download the image
greenprint "📥 Downloading the installer image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
sudo mv "$ISO_FILENAME" /var/lib/libvirt/images

# Clean compose and blueprints.
greenprint "🧹 Clean up installer blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete installer > /dev/null

##################################################################
##
## Install with simplified installer ISO
##
##################################################################
# Create qcow2 file for virt install.
greenprint "🖥 Create simplified qcow2 file for virt install"
SIMPLIFIED_LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-simplified.qcow2
sudo qemu-img create -f qcow2 "${SIMPLIFIED_LIBVIRT_IMAGE_PATH}" 20G

# Create a disk to simulate USB device to test USB installation
# New growfs service dealing with LVM in simplified installer breaks USB installation
LIBVIRT_FAKE_USB_PATH=/var/lib/libvirt/images/usb.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_FAKE_USB_PATH}" 16G

greenprint "💿 Install ostree image via embedded ignition simplified installer"
sudo virt-install  --name="${IMAGE_KEY}-simplified"\
                   --disk path="${LIBVIRT_FAKE_USB_PATH}",format=qcow2 \
                   --disk path="${SIMPLIFIED_LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 2048 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --boot "${BOOT_ARGS}" \
                   --tpm backend.type=emulator,backend.version=2.0,model=tpm-crb \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Let's detach USB disk before start VM
greenprint "💻 Detach USB disk before start VM"
sudo virsh detach-disk --domain "${IMAGE_KEY}-simplified" --target "$LIBVIRT_FAKE_USB_PATH" --persistent --config
sudo virsh vol-delete --pool images usb.qcow2

# Start VM.
greenprint "💻 Start simplified installer VM"
sudo virsh start "${IMAGE_KEY}-simplified"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $SIMPLIFIED_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Reboot one more time to make /sysroot as RO by new ostree-libs-2022.6-3.el9.x86_64
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${SIMPLIFIED_GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
# Sleep 10 seconds here to make sure vm restarted already
sleep 10
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $SIMPLIFIED_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_1_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${SIMPLIFIED_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${IGNITION_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes
ansible_become_method=sudo
ansible_become_pass=${IGNITION_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type=rhel-edge \
    -e ostree_commit="${INSTALL_HASH}" \
    -e skip_rollback_test="true" \
    -e ignition="true" \
    -e edge_type=edge-simplified-installer \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0

check_result

# Remove simplified installer ISO file
sudo rm -rf "/var/lib/libvirt/images/${ISO_FILENAME}"

##################################################################
##
## Build upgrade image
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
build_image upgrade "${CONTAINER_TYPE}" "$PROD_REPO_1_URL"

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
sudo ostree --repo="$PROD_REPO_1" pull --mirror edge-stage "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO_1" static-delta generate "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO_1" summary -u

# Get ostree commit value.
greenprint "🕹 Get ostree upgrade commit value"
UPGRADE_HASH=$(curl "${PROD_REPO_1_URL}/refs/heads/${OSTREE_REF}")

# Clean compose and blueprints.
greenprint "🧽 Clean up upgrade blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete upgrade > /dev/null

##################################################################
##
## Upgrade simplified installer VM
##
##################################################################
greenprint "🗳 Upgrade ostree image/commit on simplified VM"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${SIMPLIFIED_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |sudo -S rpm-ostree upgrade"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${SIMPLIFIED_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $SIMPLIFIED_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check ostree upgrade result
check_result

# Add instance IP address into /etc/ansible/hosts
tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${SIMPLIFIED_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${IGNITION_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes
ansible_become_method=sudo
ansible_become_pass=${IGNITION_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type=rhel-edge \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e skip_rollback_test="true" \
    -e ignition="true" \
    -e edge_type=edge-simplified-installer \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0

check_result

# Clean up VM
greenprint "🧹 Clean up simplified VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}-simplified") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-simplified"
fi
sudo virsh undefine "${IMAGE_KEY}-simplified" --nvram
sudo virsh vol-delete --pool images "$IMAGE_KEY-simplified.qcow2"

##########################################################################
##
## Build edge-simplified-installer with firtboot ignition configured
##
##########################################################################
# Write a blueprint for installer image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "installer"
description = "A rhel-edge simplified-installer image"
version = "0.0.1"
modules = []
groups = []

[customizations]
installation_device = "/dev/vda"

[customizations.ignition.firstboot]
url = "${IGNITION_SERVER_URL}/config.ign"
EOF

greenprint "📄 installer blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve installer

# Build installer image.
build_image installer "${INSTALLER_TYPE}" "${PROD_REPO_2_URL}"

# Download the image
greenprint "📥 Downloading the installer image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
sudo mv "$ISO_FILENAME" /var/lib/libvirt/images

# Clean compose and blueprints.
greenprint "🧹 Clean up installer blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete installer > /dev/null

##################################################################
##
## Install with simplified installer ISO
##
##################################################################
# Create qcow2 file for virt install.
greenprint "🖥 Create simplified qcow2 file for virt install"
SIMPLIFIED_LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-simplified.qcow2
sudo qemu-img create -f qcow2 "${SIMPLIFIED_LIBVIRT_IMAGE_PATH}" 20G

greenprint "💿 Install ostree image via firstboot ignition simplified installer"
sudo virt-install  --name="${IMAGE_KEY}-simplified"\
                   --disk path="${SIMPLIFIED_LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 2048 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-variant ${OS_VARIANT} \
                   --cdrom "/var/lib/libvirt/images/${ISO_FILENAME}" \
                   --boot "${BOOT_ARGS}" \
                   --tpm backend.type=emulator,backend.version=2.0,model=tpm-crb \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "💻 Start simplified installer VM"
sudo virsh start "${IMAGE_KEY}-simplified"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $SIMPLIFIED_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Reboot one more time to make /sysroot as RO by new ostree-libs-2022.6-3.el9.x86_64
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${SIMPLIFIED_GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
# Sleep 10 seconds here to make sure vm restarted already
sleep 10
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $SIMPLIFIED_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_2_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${SIMPLIFIED_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${IGNITION_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes
ansible_become_method=sudo
ansible_become_pass=${IGNITION_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type=rhel-edge \
    -e ostree_commit="${INSTALL_HASH}" \
    -e skip_rollback_test="true" \
    -e ignition="true" \
    -e edge_type=edge-simplified-installer \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Remove simplified installer ISO file
sudo rm -rf "/var/lib/libvirt/images/${ISO_FILENAME}"

# Clean up VM
greenprint "🧹 Clean up simplified VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}-simplified") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-simplified"
fi
sudo virsh undefine "${IMAGE_KEY}-simplified" --nvram
sudo virsh vol-delete --pool images "$IMAGE_KEY-simplified.qcow2"

# No upgrade test for ignition firstboot on simplified installer image

##################################################################
##
## Build edge-raw-image with ignition enabled
##
##################################################################

tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "raw"
description = "A rhel-edge raw image"
version = "0.0.1"
modules = []
groups = []

[customizations.ignition.firstboot]
url = "${IGNITION_SERVER_URL}/config.ign"
EOF

greenprint "📄 raw-image blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing raw-image blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve raw

# Build raw image.
build_image raw "$RAW_TYPE" "${PROD_REPO_2_URL}"

# Download raw image
greenprint "📥 Downloading the raw image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

greenprint "Extracting and converting the raw image to a qcow2 file"
RAW_FILENAME="${COMPOSE_ID}-${RAW_FILENAME}"
sudo xz -d "${RAW_FILENAME}"
RAW_LIBVIRT_IMAGE_PATH="/var/lib/libvirt/images/${IMAGE_KEY}-raw.qcow2"
sudo qemu-img convert -f raw "${COMPOSE_ID}-image.raw" -O qcow2 "$RAW_LIBVIRT_IMAGE_PATH"
# Remove raw file
sudo rm -f "${COMPOSE_ID}-image.raw"

# Clean compose and blueprints.
greenprint "🧹 Clean up raw blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete raw > /dev/null

##################################################################
##
## Install with raw image
##
##################################################################

greenprint "💿 Install ostree image via raw image on UEFI VM"
sudo virt-install  --name="${IMAGE_KEY}-raw"\
                   --disk path="${RAW_LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 2048 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:31 \
                   --os-variant ${OS_VARIANT} \
                   --boot "${BOOT_ARGS}" \
                   --tpm backend.type=emulator,backend.version=2.0,model=tpm-crb \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --import \
                   --noreboot

# Start VM.
greenprint "💻 Start UEFI VM"
sudo virsh start "${IMAGE_KEY}-raw"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $RAW_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Reboot one more time to make /sysroot as RO by new ostree-libs-2022.6-3.el9.x86_64
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${RAW_GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
# Sleep 10 seconds here to make sure vm restarted already
sleep 10
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $RAW_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_2_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${RAW_GUEST_ADDRESS}
[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${IGNITION_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes
ansible_become_method=sudo
ansible_become_pass=${IGNITION_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type=rhel-edge \
    -e ostree_commit="${INSTALL_HASH}" \
    -e skip_rollback_test="true" \
    -e ignition="true" \
    -e edge_type=edge-raw-image \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0

check_result

# Pull upgrade to prod mirror
greenprint "⛓ Pull upgrade to prod mirror"
sudo ostree --repo="$PROD_REPO_2" pull --mirror edge-stage "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO_2" static-delta generate "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO_2" summary -u

# Clean upgrade container
sudo podman rm -f rhel-edge
sudo podman rmi -f "$EDGE_IMAGE_ID"

# Get ostree commit value.
greenprint "🕹 Get ostree upgrade commit value"
UPGRADE_HASH=$(curl "${PROD_REPO_2_URL}/refs/heads/${OSTREE_REF}")

##################################################################
##
## Upgrade raw image VM
##
##################################################################
greenprint "🗳 Upgrade ostree image/commit on raw image VM"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${RAW_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |sudo -S rpm-ostree upgrade"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${RAW_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $RAW_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check ostree upgrade result
check_result

# Add instance IP address into /etc/ansible/hosts
tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${RAW_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${IGNITION_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes
ansible_become_method=sudo
ansible_become_pass=${IGNITION_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type=rhel-edge \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e skip_rollback_test="true" \
    -e ignition="true" \
    -e edge_type=edge-raw-image \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0

check_result

# Clean up VM
greenprint "🧹 Clean up raw image VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}-raw") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}-raw"
fi
sudo virsh undefine "${IMAGE_KEY}-raw" --nvram
sudo virsh vol-delete --pool images "$IMAGE_KEY-raw.qcow2"

# Final success clean up
clean_up

exit 0
