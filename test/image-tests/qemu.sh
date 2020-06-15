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

# We need jq for parsing composer-cli output.
if ! hash jq; then
    greenprint "📦 Installing jq"
    sudo dnf -qy install jq
fi

# Install required packages.
greenprint "📦 Installing required packages"
sudo dnf -qy install htop libvirt-client libvirt-daemon \
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

# Allow anyone in the wheel group to talk to libvirt.
greenprint "🚪 Allowing users in wheel group to valk to libvirt"
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

# Jenkins sets WORKSPACE to the job workspace, but if this script runs
# outside of Jenkins, we can set up a temporary directory instead.
if [[ ${WORKSPACE:-empty} == empty ]]; then
    WORKSPACE=$(mktemp -d)
fi

# Set up variables.
TEST_UUID=$(uuidgen)
IMAGE_KEY=osbuild-composer-aws-test-${TEST_UUID}

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json
DOMIF_CHECK=${TEMPDIR}/domifcheck-${IMAGE_KEY}.json

# Check for the smoke test file on the AWS instance that we start.
smoke_test_check () {
    # Ensure the ssh key has restricted permissions.
    SSH_KEY=${WORKSPACE}/test/keyring/id_rsa
    chmod 0600 $SSH_KEY

    SMOKE_TEST=$(ssh -i ${SSH_KEY} redhat@${1} 'cat /etc/smoke-test.txt')
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
    sleep 30
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

# Set up a cloud-init ISO.
greenprint "💿 Creating a cloud-init ISO"
CLOUD_INIT_PATH=/var/lib/libvirt/images/seed.iso
cp ${WORKSPACE}/test/cloud-init/*-data .
sudo genisoimage -o $CLOUD_INIT_PATH -V cidata -r -J user-data meta-data 2>&1 > /dev/null

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
    --network network=default

# Wait for the image to make a DHCP request.
greenprint "💻 Waiting for the instance to make a DHCP request."
for LOOP_COUNTER in {0..30}; do
    sudo virsh domifaddr ${IMAGE_KEY} | tee $DOMIF_CHECK > /dev/null

    # Check to see if the CIDR IP is in the output yet.
    if grep -oP "[0-9\.]*/[0-9]*" $DOMIF_CHECK > /dev/null; then
        INSTANCE_ADDRESS=$(grep -oP "[0-9\.]*/[0-9]*" $DOMIF_CHECK | sed 's#/.*$##')
        echo "Found instance address: ${INSTANCE_ADDRESS}"
        break
    fi

    # Wait 10 seconds and try again.
    sleep 10
done

# Wait for SSH to start.
greenprint "⏱ Waiting for instance to respond to ssh"
for LOOP_COUNTER in {0..30}; do
    if ssh-keyscan $INSTANCE_ADDRESS 2>&1 > /dev/null; then
        echo "SSH is up!"
        ssh-keyscan $INSTANCE_ADDRESS >> ~/.ssh/known_hosts
        break
    fi

    # ssh-keyscan has a 5 second timeout by default, so the pause per loop
    # is 10 seconds when you include the following `sleep`.
    echo "Retrying in 5 seconds..."
    sleep 5
done

# Check for our smoke test file.
greenprint "🛃 Checking for smoke test file"
for LOOP_COUNTER in {0..10}; do
    RESULTS="$(smoke_test_check $INSTANCE_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "Smoke test passed! 🥳"
        break
    fi
    sleep 5
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
