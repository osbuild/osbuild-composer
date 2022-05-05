#!/usr/bin/bash
set -euo pipefail

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Get OS data.
source /etc/os-release

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
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

TEST_UUID=$(uuidgen)
IMAGE_KEY="oscap-${TEST_UUID}"
HTTP_GUEST_ADDRESS=192.168.100.50
ARTIFACTS="ci-artifacts"
mkdir -p "${ARTIFACTS}"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_USER="admin"
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)

case "${ID}-${VERSION_ID}" in
    "rhel-8.7")
        OS_VARIANT="rhel8-unknown"
        PROFILE="xccdf_org.ssgproject.content_profile_cis"
        DATASTREAM="/usr/share/xml/scap/ssg/content/ssg-rhel8-ds.xml"
        ;;
    "rhel-9.1")
        OS_VARIANT="rhel9-unknown"
        PROFILE="xccdf_org.ssgproject.content_profile_cis"
        DATASTREAM="/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml"
        ;;
    "centos-8")
        OS_VARIANT="centos8"
        PROFILE="xccdf_org.ssgproject.content_profile_pci-dss"
        DATASTREAM="/usr/share/xml/scap/ssg/content/ssg-centos8-ds.xml"
        ;;
    "centos-9")
        OS_VARIANT="centos-stream9"
        PROFILE="xccdf_org.ssgproject.content_profile_cis"
        DATASTREAM="/usr/share/xml/scap/ssg/content/ssg-cs9-ds.xml"
        ;;
    "fedora-35")
        OS_VARIANT="fedora-unknown"
        PROFILE="xccdf_org.ssgproject.content_profile_standard"
        DATASTREAM="/usr/share/xml/scap/ssg/content/ssg-fedora-ds.xml"
        ;;
    *)
        echo "$0 is not enabled for ${ID}-${VERSION_ID} skipping..."
        exit 0
        ;;
esac

# Insall necessary packages to run
# scans and check results
sudo dnf install -y \
xmlstarlet \
openscap-utils \
scap-security-guide 

get_build_info() {
    key="$1"
    fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
    fi
    jq -r "${key}" "${fname}"
}

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "${COMPOSE_ID}" | tee "${LOG_FILE}" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.json

    # Download the metadata.
    sudo composer-cli compose metadata "${COMPOSE_ID}" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "${TARBALL}" -C "${TEMPDIR}"
    sudo rm -f "${TARBALL}"

    # Move the JSON file into place.
    sudo cat "${TEMPDIR}"/"${COMPOSE_ID}".json | jq -M '.' | tee "${METADATA_FILE}" > /dev/null
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
    sudo composer-cli --json compose start "${blueprint_name}" "${image_type}" | tee "${COMPOSE_START}"
    COMPOSE_ID=$(get_build_info ".build_id" "${COMPOSE_START}")

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "${COMPOSE_INFO}" > /dev/null
        COMPOSE_STATUS=$(get_build_info ".queue_status" "${COMPOSE_INFO}")

        # Is the compose finished?
        if [[ ${COMPOSE_STATUS} != RUNNING ]] && [[ ${COMPOSE_STATUS} != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 5
    done

    # Capture the compose logs from osbuild.
    greenprint "💬 Getting compose log and metadata"
    get_compose_log "${COMPOSE_ID}"
    get_compose_metadata "${COMPOSE_ID}"

    # Kill the journal monitor immediately and remove the trap
    sudo pkill -P "${WORKER_JOURNAL_PID}"
    trap - EXIT

    # Did the compose finish with success?
    if [[ ${COMPOSE_STATUS} != FINISHED ]]; then
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi
}

# Wait for the ssh server up to be.
wait_for_ssh_up () {
    SSH_STATUS=$(sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${1}" '/bin/bash -c "echo -n READY"')
    if [[ ${SSH_STATUS} == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

scan_vm() {
  # oscap package has been installed on vm as
  # oscap customization has been specified
  sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${1}" \
    oscap xccdf eval \
    --results report.xml \
    --profile "${PROFILE}" \
    "${DATASTREAM}" || true # oscap returns exit code 2 for any failed rules
}

# Clean up our mess.
clean_up () {
    # Clear vm
    if [[ $(sudo virsh domstate "${VIRSH_DOMAIN}") == "running" ]]; then
        sudo virsh destroy "${VIRSH_DOMAIN}"
    fi
    sudo virsh undefine "${VIRSH_DOMAIN}" --nvram

    # Remove qcow2 file.
    sudo virsh vol-delete --pool images "${LIBVIRT_IMAGE_PATH}"
}

###############################
##
## Baseline Image
##
###############################

# Write a blueprint for baseline image.
tee "${BLUEPRINT_FILE}" > /dev/null << EOF
name = "baseline"
description = "A base image"
version = "0.0.1"
modules = []
groups = []

[[packages]]
name = "openscap-scanner"
version = "*"

[[packages]]
name = "scap-security-guide"
version = "*"

[[customizations.user]]
name = "${SSH_USER}"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/${SSH_USER}/"
groups = ["wheel"]
EOF

greenprint "📄 baseline blueprint"
cat "${BLUEPRINT_FILE}"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing baseline blueprint"
sudo composer-cli blueprints push "${BLUEPRINT_FILE}"
sudo composer-cli blueprints depsolve baseline

# Build baseline image.
build_image baseline qcow2

# Download the image
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "🖥 Create qcow2 file for virt install"
sudo cp "${IMAGE_FILENAME}" "/var/lib/libvirt/images/${IMAGE_KEY}.qcow2"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.qcow2
sudo qemu-img resize "${LIBVIRT_IMAGE_PATH}" 20G

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

greenprint "📋 Install baseline image"
VIRSH_DOMAIN="${IMAGE_KEY}-baseline"
sudo virt-install --name="${VIRSH_DOMAIN}"\
                  --disk "${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                  --ram 3072 \
                  --vcpus 2 \
                  --network network=integration,mac=34:49:22:B0:83:30 \
                  --os-type linux \
                  --os-variant "${OS_VARIANT}" \
                  --import \
                  --noautoconsole \
                  --nographics \
                  --wait=15 \
                  --noreboot

# Installation can get stuck, destroying VM helps
# See https://github.com/osbuild/osbuild-composer/issues/2413
if [[ $(sudo virsh domstate "${VIRSH_DOMAIN}") == "running" ]]; then
    sudo virsh destroy "${VIRSH_DOMAIN}"
fi

# Start VM.
greenprint "💻 Start baseline VM"
sudo virsh start "${VIRSH_DOMAIN}"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 45); do
    RESULTS="$(wait_for_ssh_up ${HTTP_GUEST_ADDRESS})"
    if [[ ${RESULTS} == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 20
done

greenprint "🔒 Running oscap scanner"
scan_vm "${HTTP_GUEST_ADDRESS}"

greenprint "📗 Checking oscap score"
BASELINE_SCORE=$(sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${HTTP_GUEST_ADDRESS}" cat report.xml | xmlstarlet sel -N x="http://checklists.nist.gov/xccdf/1.2" -t -v "//x:score")

# Clean up BASELINE VM
greenprint "🧹 Clean up VM"
clean_up

###############################
##
## Hardened Image
##
###############################

# Write a blueprint for hardened image.
tee "${BLUEPRINT_FILE}" > /dev/null << EOF
name = "hardened"
description = "A hardened OpenSCAP image"
version = "0.0.1"
modules = []
groups = []

[customizations.openscap]
data_dir = "/home/${SSH_USER}/oscap_data"
profile_id = "${PROFILE}"

[[customizations.user]]
name = "${SSH_USER}"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/${SSH_USER}/"
groups = ["wheel"]
EOF

greenprint "📄 hardened blueprint"
cat "${BLUEPRINT_FILE}"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing hardened blueprint"
sudo composer-cli blueprints push "${BLUEPRINT_FILE}"
sudo composer-cli blueprints depsolve hardened

# Build hardened image.
build_image hardened qcow2

# Download the image
greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"

greenprint "🖥 Create qcow2 file for virt install"
sudo cp "${IMAGE_FILENAME}" "/var/lib/libvirt/images/${IMAGE_KEY}.qcow2"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.qcow2
sudo qemu-img resize "${LIBVIRT_IMAGE_PATH}" 20G

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

greenprint "📋 Install hardened image"
VIRSH_DOMAIN="${IMAGE_KEY}-hardened"
sudo virt-install --name="${VIRSH_DOMAIN}"\
                  --disk "${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                  --ram 3072 \
                  --vcpus 2 \
                  --network network=integration,mac=34:49:22:B0:83:30 \
                  --os-type linux \
                  --os-variant "${OS_VARIANT}" \
                  --import \
                  --nographics \
                  --noautoconsole \
                  --wait=15 \
                  --noreboot

# Installation can get stuck, destroying VM helps
# See https://github.com/osbuild/osbuild-composer/issues/2413
if [[ $(sudo virsh domstate "${VIRSH_DOMAIN}") == "running" ]]; then
    sudo virsh destroy "${VIRSH_DOMAIN}"
fi

# Start VM.
greenprint "💻 Start hardened VM"
sudo virsh start "${VIRSH_DOMAIN}"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for LOOP_COUNTER in $(seq 0 45); do
    RESULTS="$(wait_for_ssh_up ${HTTP_GUEST_ADDRESS})"
    if [[ ${RESULTS} == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    # higher timeout for first-boot remediation to complete
    sleep 20 
done

greenprint "📗 Checking oscap score"
HARDENED_SCORE=$(sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${HTTP_GUEST_ADDRESS}" \
  cat oscap_data/eval_remediate_results.xml \
  | xmlstarlet sel -N x="http://checklists.nist.gov/xccdf/1.2" -t -v "//x:score"
)

# Clean up HARDENED VM
greenprint "🧹 Clean up VM"
clean_up

# Remomve tmp dir.
sudo rm -rf "${TEMPDIR}"

greenprint "🎏 Checking for test result"
echo "Baseline score: ${BASELINE_SCORE}%"
echo "Hardened score: ${HARDENED_SCORE}%"

# compare floating point numbers
if python3 -c "exit(not(${HARDENED_SCORE} > ${BASELINE_SCORE}))"; then
    greenprint "💚 Success"
else
    greenprint "❌ Failed"
    exit 1
fi