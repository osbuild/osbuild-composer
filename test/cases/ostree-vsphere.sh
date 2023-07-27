#!/bin/bash

# This test scrip covers:
# 1. build edge-container image, build edge-vsphere image
# 2. upload image to vsphere datacenter7.0-amd, create edge vm, run test cases against vm.
# 3. upload image to vsphere datacenter8.0-intel, create edge vm, run test cases against vm.
# 4. build upgrade edge-container image
# 5. run rpm-ostree upgrade in all edge vms, then run test cases against upgraded vms.
# 6. delete all vsphere vms, images

set -euo pipefail

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Install sshpass
sudo dnf install -y sshpass

# Install ansible module
sudo ansible-galaxy collection install community.vmware
sudo pip3 install PyVmomi

# Install govc
sudo curl -L -o - "https://github.com/vmware/govmomi/releases/latest/download/govc_$(uname -s)_$(uname -m).tar.gz" | tar -C /usr/local/bin -xvzf - govc

# Start firewall
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
      <host mac='34:49:22:B0:83:30' name='vm' ip='192.168.100.50'/>
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

# Stop firewalld, otherwise vsphere vm cannot access local ignition config files
sudo systemctl disable --now firewalld

# Set up variables.
TEST_UUID=$(uuidgen)
IMAGE_KEY="edge-${TEST_UUID}"
HOST_IP_ADDRESS=$(ip addr show "$(ip route | awk '/default/ { print $5 }')" | grep "inet" | head -n 1 | awk '/inet/ {print $2}' | cut -d'/' -f1)
PROD_REPO_URL=http://${HOST_IP_ADDRESS}/repo
PROD_REPO=/var/www/html/repo
STAGE_REPO_ADDRESS=192.168.200.1
STAGE_REPO_URL="http://${STAGE_REPO_ADDRESS}:8080/repo/"
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
CONTAINER_TYPE=edge-container
CONTAINER_FILENAME=container.tar
INSTALLER_TYPE=edge-vsphere
INSTALLER_FILENAME=image.vmdk

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

# Ignition setup
IGNITION_SERVER_FOLDER=/var/www/html/ignition
IGNITION_SERVER_URL=http://${HOST_IP_ADDRESS}/ignition
IGNITION_USER=core
IGNITION_USER_PASSWORD=foobar

case "${ID}-${VERSION_ID}" in
    "rhel-9"* )
        OSTREE_REF="rhel/9/${ARCH}/edge"
        SYSROOT_RO="true"
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
    SSH_STATUS=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}"@"${1}" '/bin/bash -c "echo -n READY"')
    if [[ $SSH_STATUS == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

# Clean up our mess.
clean_up () {
    greenprint "🧼 Cleaning up"

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

##############################################################
##
## Build edge-container image
##
##############################################################
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
name = "open-vm-tools"
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
sudo ostree --repo="$PROD_REPO" pull --mirror edge-stage "$OSTREE_REF"

# Clean compose and blueprints.
greenprint "🧽 Clean up container blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete container > /dev/null

##################################################################
##
## Generate ignition configuration
##
##################################################################
greenprint "📋 Preparing ignition environment"
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

##################################################################
##
## Build edge-vsphere with Ignition firstboot
##
##################################################################
greenprint "📋 Build edge-vsphere image"
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "vmdk"
description = "A rhel-edge vmdk image"
version = "0.0.1"
modules = []
groups = []

[customizations.ignition.firstboot]
url = "${IGNITION_SERVER_URL}/config.ign"
EOF

greenprint "📄 vmdk blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing installer blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve vmdk

# Build simplified installer iso image.
build_image vmdk "${INSTALLER_TYPE}" "${PROD_REPO_URL}/"

# Download the image
greenprint "📥 Downloading the vmdk image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
ISO_FILENAME="${COMPOSE_ID}-${INSTALLER_FILENAME}"
sudo mv ${ISO_FILENAME} edge-vsphere.vmdk

# Clean compose and blueprints.
greenprint "🧹 Clean up vmdk blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete vmdk > /dev/null

##################################################################
##
## Create vm on datacenter7.0-amd and test it
##
##################################################################
# Datacenter setup
VSPHERE_DC70="Datacenter7.0-AMD"
VSPHERE_DC70_HOST="10.73.196.21"
VSPHERE_DC70_DATASTORE="datastore-21"
DC70_VSPHERE_VM_NAME="${COMPOSE_ID}-70"

# Remove all existing disk files
greenprint "🧹 Remove existing vmdk disk files in vsphere datacenter"
sudo sshpass -p "${DATACENTER_PASSWORD}" ssh "${SSH_OPTIONS[@]}" "${DATACENTER_USERNAME}@${VSPHERE_DC70_HOST}" \
    rm -fr /vmfs/volumes/${VSPHERE_DC70_DATASTORE}/edge-vsphere

# Upload image to vsphere and create VM
export GOVC_URL="https://bootp-73-75-144.lab.eng.pek2.redhat.com/sdk"
export GOVC_USERNAME="${VSPHERE_USERNAME}"
export GOVC_PASSWORD="${VSPHERE_PASSWORD}"
export GOVC_INSECURE="true"

greenprint "📋 Uploading vmdk image to vsphere datacenter 7.0"
export GOVC_DATACENTER="Datacenter7.0-AMD"
govc import.vmdk -ds="datastore-21" -pool="/Datacenter7.0-AMD/host/Cluster7.0-AMD/Resources" ./edge-vsphere.vmdk

# Create vm with vmdk image
greenprint "📋 Create vm in vsphere datacenter"
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e vsphere_host="${VSPHERE_HOST}" \
    -e vsphere_username="${VSPHERE_USERNAME}" \
    -e vsphere_password="${VSPHERE_PASSWORD}" \
    -e vsphere_datacenter="${VSPHERE_DC70}" \
    -e vsphere_esxi_host="${VSPHERE_DC70_HOST}" \
    -e vsphere_datastore="${VSPHERE_DC70_DATASTORE}" \
    -e vsphere_vm_name="${DC70_VSPHERE_VM_NAME}" \
    /usr/share/tests/osbuild-composer/ansible/vmdk-create-vm.yaml || RESULTS=0

DC70_GUEST_ADDRESS=$(cat /usr/share/tests/osbuild-composer/ansible/vmdk_ip.txt)
sudo rm -fr /usr/share/tests/osbuild-composer/ansible/vmdk_ip.txt
greenprint "🛃 Edge VM IP address is: ${DC70_GUEST_ADDRESS}"

# Run ansible check on edge vm
# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $DC70_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# With new ostree-libs-2022.6-3, edge vm needs to reboot twice to make the /sysroot readonly
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${DC70_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"
# Sleep 10 seconds here to make sure vm restarted already
sleep 10
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $DC70_GUEST_ADDRESS)"
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

sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${DC70_GUEST_ADDRESS}

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
    -e image_type=redhat \
    -e ostree_commit="${INSTALL_HASH}" \
    -e skip_rollback_test="true" \
    -e edge_type=edge-vsphere \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

##################################################################
##
## Create vm on datacenter8.0 and test it
##
##################################################################
# Vsphere setup
VSPHERE_DC80="Datacenter8.0"
VSPHERE_DC80_HOST="10.73.227.56"
VSPHERE_DC80_DATASTORE="datastore-56"
DC80_VSPHERE_VM_NAME="${COMPOSE_ID}-80"

# Remove all existing disk files
greenprint "🧹 Remove existing vmdk disk files in vsphere datacenter"
sudo sshpass -p "${DATACENTER_PASSWORD}" ssh "${SSH_OPTIONS[@]}" "${DATACENTER_USERNAME}@${VSPHERE_DC80_HOST}" \
    rm -fr /vmfs/volumes/${VSPHERE_DC80_DATASTORE}/edge-vsphere

greenprint "📋 Uploading vmdk image to vsphere datacenter 8.0"
export GOVC_DATACENTER="Datacenter8.0"
govc import.vmdk -ds="datastore-56" -pool="/Datacenter8.0/host/Cluster8.0/Resources" ./edge-vsphere.vmdk

# Create vm with vmdk image
greenprint "📋 Create vm in vsphere datacenter"
tee "${TEMPDIR}"/inventory > /dev/null << EOF
localhost ansible_connection=local
EOF
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e vsphere_host="${VSPHERE_HOST}" \
    -e vsphere_username="${VSPHERE_USERNAME}" \
    -e vsphere_password="${VSPHERE_PASSWORD}" \
    -e vsphere_datacenter="${VSPHERE_DC80}" \
    -e vsphere_esxi_host="${VSPHERE_DC80_HOST}" \
    -e vsphere_datastore="${VSPHERE_DC80_DATASTORE}" \
    -e vsphere_vm_name="${DC80_VSPHERE_VM_NAME}" \
    /usr/share/tests/osbuild-composer/ansible/vmdk-create-vm.yaml || RESULTS=0

DC80_GUEST_ADDRESS=$(cat /usr/share/tests/osbuild-composer/ansible/vmdk_ip.txt)
sudo rm -fr /usr/share/tests/osbuild-composer/ansible/vmdk_ip.txt
greenprint "🛃 Edge VM IP address is: ${DC80_GUEST_ADDRESS}"

# Run ansible check on edge vm
# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $DC80_GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# With new ostree-libs-2022.6-3, edge vm needs to reboot twice to make the /sysroot readonly
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${DC80_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"
# Sleep 10 seconds here to make sure vm restarted already
sleep 10
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $DC80_GUEST_ADDRESS)"
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

sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${DC80_GUEST_ADDRESS}

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
    -e image_type=redhat \
    -e ostree_commit="${INSTALL_HASH}" \
    -e skip_rollback_test="true" \
    -e edge_type=edge-vsphere \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

##################################################################
##
## Build upgrade edge-vsphere image
##
##################################################################
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
name = "open-vm-tools"
version = "*"

[[packages]]
name = "wget"
version = "*"
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

##################################################################
##
## Run upgrade test on datacenter7.0 amd
##
##################################################################
greenprint "🗳 Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${DC70_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |sudo -S rpm-ostree upgrade"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${DC70_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $DC70_GUEST_ADDRESS)"
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
${DC70_GUEST_ADDRESS}

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
    -e image_type=redhat \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e skip_rollback_test="true" \
    -e edge_type=edge-vsphere \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0

check_result

##################################################################
##
## Run upgrade test on datacenter8.0 intel
##
##################################################################
greenprint "🗳 Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${DC80_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |sudo -S rpm-ostree upgrade"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${IGNITION_USER}@${DC80_GUEST_ADDRESS}" "echo ${IGNITION_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure vm restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for _ in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $DC80_GUEST_ADDRESS)"
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
${DC80_GUEST_ADDRESS}

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
    -e image_type=redhat \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e skip_rollback_test="true" \
    -e edge_type=edge-vsphere \
    -e fdo_credential="false" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result
##################################################################
##
## Remove vm in vsphere
##
##################################################################
greenprint "📋 Poweroff and remove all vms"
tee "${TEMPDIR}"/inventory > /dev/null << EOF
localhost ansible_connection=local
EOF
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e vsphere_host="${VSPHERE_HOST}" \
    -e vsphere_username="${VSPHERE_USERNAME}" \
    -e vsphere_password="${VSPHERE_PASSWORD}" \
    -e vsphere_datacenter="${VSPHERE_DC70}" \
    -e vsphere_esxi_host="${VSPHERE_DC70_HOST}" \
    -e vsphere_datastore="${VSPHERE_DC70_DATASTORE}" \
    -e vsphere_vm_name="${DC70_VSPHERE_VM_NAME}" \
    /usr/share/tests/osbuild-composer/ansible/vmdk-remove-vm.yaml || RESULTS=0

sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e vsphere_host="${VSPHERE_HOST}" \
    -e vsphere_username="${VSPHERE_USERNAME}" \
    -e vsphere_password="${VSPHERE_PASSWORD}" \
    -e vsphere_datacenter="${VSPHERE_DC80}" \
    -e vsphere_esxi_host="${VSPHERE_DC80_HOST}" \
    -e vsphere_datastore="${VSPHERE_DC80_DATASTORE}" \
    -e vsphere_vm_name="${DC80_VSPHERE_VM_NAME}" \
    /usr/share/tests/osbuild-composer/ansible/vmdk-remove-vm.yaml || RESULTS=0

# Remove all existing disk files
greenprint "🧹 Remove existing vmdk disk files in vsphere datacenter"
sudo sshpass -p "${DATACENTER_PASSWORD}" ssh "${SSH_OPTIONS[@]}" "${DATACENTER_USERNAME}@${VSPHERE_DC70_HOST}" \
    rm -fr /vmfs/volumes/${VSPHERE_DC70_DATASTORE}/edge-vsphere
sudo sshpass -p "${DATACENTER_PASSWORD}" ssh "${SSH_OPTIONS[@]}" "${DATACENTER_USERNAME}@${VSPHERE_DC80_HOST}" \
    rm -fr /vmfs/volumes/${VSPHERE_DC80_DATASTORE}/edge-vsphere
# Final success clean up
clean_up

exit 0
