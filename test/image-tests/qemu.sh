#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release

# Take the image type passed to the script or use qcow2 by default if nothing
# was passed.
IMAGE_TYPE=${1:-qcow2}

# Select the file extension based on the image that we are building.
IMAGE_EXTENSION=$IMAGE_TYPE
if [[ $IMAGE_TYPE == 'openstack' ]]; then
    IMAGE_EXTENSION=qcow2
fi

# RHEL 8 cannot boot a VMDK using libvirt. See BZ 999789.
if [[ $IMAGE_TYPE == vmdk ]] && [[ $ID == rhel ]]; then
    echo "🤷 RHEL 8 cannot boot a VMDK. See BZ 999789."
    exit 0
fi

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

# Install required packages.
greenprint "📦 Installing required packages"
sudo dnf -y install jq libvirt-client libvirt-daemon \
    libvirt-daemon-config-network libvirt-daemon-config-nwfilter \
    libvirt-daemon-driver-interface libvirt-daemon-driver-network \
    libvirt-daemon-driver-nodedev libvirt-daemon-driver-nwfilter \
    libvirt-daemon-driver-qemu libvirt-daemon-driver-secret \
    libvirt-daemon-driver-storage libvirt-daemon-driver-storage-disk \
    libvirt-daemon-kvm qemu-img qemu-kvm virt-install

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
IMAGE_KEY=osbuild-composer-aws-test-${TEST_UUID}
INSTANCE_ADDRESS=192.168.100.50

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# Check for the smoke test file on the AWS instance that we start.
smoke_test_check () {
    # Ensure the ssh key has restricted permissions.
    SSH_KEY=${WORKSPACE}/test/keyring/id_rsa
    chmod 0600 $SSH_KEY

    SSH_OPTIONS="-o StrictHostKeyChecking=no -o ConnectTimeout=5"
    SMOKE_TEST=$(ssh $SSH_OPTIONS -i ${SSH_KEY} redhat@${1} 'cat /etc/smoke-test.txt')
    if [[ $SMOKE_TEST == smoke-test ]]; then
        echo 1
    else
        echo 0
    fi
}

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-${IMAGE_TYPE}.log

    # Download the logs.
    sudo composer-cli compose log $COMPOSE_ID | tee $LOG_FILE > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${WORKSPACE}/osbuild-${ID}-${VERSION_ID}-${IMAGE_TYPE}.json

    # Download the metadata.
    sudo composer-cli compose metadata $COMPOSE_ID > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename $(find . -maxdepth 1 -type f -name "*-metadata.tar"))
    tar -xf $TARBALL
    rm -f $TARBALL

    # Move the JSON file into place.
    cat ${COMPOSE_ID}.json | jq -M '.' | tee $METADATA_FILE > /dev/null
}

# Write a basic blueprint for our image.
# NOTE(mhayden): The service customization will always be required for QCOW2
# but it is needed for OpenStack due to issue #698 in osbuild-composer. 😭
# NOTE(mhayden): The cloud-init package isn't included in VHD/Azure images
# by default and it must be added here.
tee $BLUEPRINT_FILE > /dev/null << EOF
name = "bash"
description = "A base system with bash"
version = "0.0.1"

[[packages]]
name = "bash"

[[packages]]
name = "cloud-init"

[customizations.services]
enabled = ["sshd", "cloud-init", "cloud-init-local", "cloud-config", "cloud-final"]

[customizations.kernel]
append = "LANG=en_US.UTF-8 net.ifnames=0 biosdevname=0"
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push $BLUEPRINT_FILE
sudo composer-cli blueprints depsolve bash

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | egrep -o "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u ${WORKER_UNIT} &
WORKER_JOURNAL_PID=$!

# Start the compose
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start bash $IMAGE_TYPE | tee $COMPOSE_START
COMPOSE_ID=$(jq -r '.build_id' $COMPOSE_START)

# Wait for the compose to finish.
greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info ${COMPOSE_ID} | tee $COMPOSE_INFO > /dev/null
    COMPOSE_STATUS=$(jq -r '.queue_status' $COMPOSE_INFO)

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 5
done

# Capture the compose logs from osbuild.
greenprint "💬 Getting compose log and metadata"
get_compose_log $COMPOSE_ID
get_compose_metadata $COMPOSE_ID

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi

# Stop watching the worker journal.
sudo kill ${WORKER_JOURNAL_PID}

# Download the image.
greenprint "📥 Downloading the image"
sudo composer-cli compose image ${COMPOSE_ID} > /dev/null
IMAGE_FILENAME=$(basename $(find . -maxdepth 1 -type f -name "*.${IMAGE_EXTENSION}"))
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.${IMAGE_EXTENSION}
sudo mv $IMAGE_FILENAME $LIBVIRT_IMAGE_PATH

# Prepare cloud-init data.
CLOUD_INIT_DIR=$(mktemp -d)
cp ${WORKSPACE}/test/cloud-init/{meta,user}-data ${CLOUD_INIT_DIR}/
cp ${WORKSPACE}/test/cloud-init/network-config ${CLOUD_INIT_DIR}/

# Set up a cloud-init ISO.
greenprint "💿 Creating a cloud-init ISO"
CLOUD_INIT_PATH=/var/lib/libvirt/images/seed.iso
rm -f $CLOUD_INIT_PATH
pushd $CLOUD_INIT_DIR
    sudo genisoimage -o $CLOUD_INIT_PATH -V cidata \
        -r -J user-data meta-data network-config 2>&1 > /dev/null
popd

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Run virt-install to import the QCOW and boot it.
greenprint "🚀 Booting the image with libvirt"
sudo virt-install \
    --name $IMAGE_KEY \
    --memory 2048 \
    --vcpus 2 \
    --disk path=${LIBVIRT_IMAGE_PATH} \
    --disk path=${CLOUD_INIT_PATH},device=cdrom \
    --import \
    --os-variant rhel8-unknown \
    --noautoconsole \
    --network network=integration,mac=34:49:22:B0:83:30 > /dev/null

# Check for our smoke test file.
greenprint "🛃 Checking for smoke test file in VM"
for LOOP_COUNTER in {0..30}; do
    RESULTS="$(smoke_test_check $INSTANCE_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "Smoke test passed! 🥳"
        break
    fi
    sleep 10
done

# Clean up our mess.
greenprint "🧼 Cleaning up"
sudo virsh destroy ${IMAGE_KEY}
sudo virsh undefine ${IMAGE_KEY}
sudo rm -f $LIBVIRT_IMAGE_PATH $CLOUD_INIT_PATH

# Use the return code of the smoke test to determine if we passed or failed.
if [[ $RESULTS == 1 ]]; then
  greenprint "💚 Success"
else
  greenprint "❌ Failed"
  exit 1
fi

exit 0
