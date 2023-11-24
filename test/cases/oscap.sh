#!/usr/bin/bash
set -euo pipefail

OSBUILD_COMPOSER_TEST_DATA=/usr/share/tests/osbuild-composer/

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Get OS data.
source /etc/os-release

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# workaround for bug https://bugzilla.redhat.com/show_bug.cgi?id=2213660
if [[ "$VERSION_ID" == "9.3"  || "$VERSION_ID" == "9" ]]; then
    sudo tee /etc/sysconfig/libvirtd << EOF > /dev/null
LIBVIRTD_ARGS=
EOF
fi

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
# create artifacts folder
ARTIFACTS="${ARTIFACTS:=/tmp/artifacts}"
mkdir -p "${ARTIFACTS}"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json
USER_DATA_FILE=${TEMPDIR}/user-data

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_USER="admin"
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)

case "${ID}-${VERSION_ID}" in
    "rhel-8"* )
        OS_VARIANT="rhel8-unknown"
        PROFILE="xccdf_org.ssgproject.content_profile_cis"
        DATASTREAM="/usr/share/xml/scap/ssg/content/ssg-rhel8-ds.xml"
        ;;
    "rhel-9"* )
        OS_VARIANT="rhel9-unknown"
        PROFILE="xccdf_org.ssgproject.content_profile_cis"
        DATASTREAM="/usr/share/xml/scap/ssg/content/ssg-rhel9-ds.xml"
        ;;
    *)
        redprint "$0 should be skipped on ${ID}-${VERSION_ID} check gitlab-ci.yml"
        exit 1
        ;;
esac

tee "${USER_DATA_FILE}" > /dev/null << EOF
#cloud-config
users:
    - name: "${SSH_USER}"
      sudo: "ALL=(ALL) NOPASSWD:ALL"
      ssh_authorized_keys:
          - "${SSH_KEY_PUB}"
EOF

# Prepare cloud-init data.
CLOUD_INIT_DIR=$(mktemp -d)
cp "${OSBUILD_COMPOSER_TEST_DATA}"/cloud-init/meta-data "${CLOUD_INIT_DIR}"/
cp "${USER_DATA_FILE}" "${CLOUD_INIT_DIR}"/

# Set up a cloud-init ISO.
gen_iso () {
    CLOUD_INIT_PATH=/var/lib/libvirt/images/seed.iso
    sudo rm -f "${CLOUD_INIT_PATH}"
    pushd "${CLOUD_INIT_DIR}"
      sudo mkisofs -o "${CLOUD_INIT_PATH}" -V cidata \
        -r -J user-data meta-data > /dev/null 2>&1
    popd
}

# Insall necessary packages to run
# scans and check results
sudo dnf install -y xmlstarlet openscap-utils scap-security-guide

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
        redprint "Something went wrong with the compose. 😢"
        exit 1
    fi
}

ssh_ready () {
    SSH_STATUS=$(sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${1}" '/bin/bash -c "echo -n READY"')
    if [[ ${SSH_STATUS} == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

wait_for_ssh_up () {
  # shellcheck disable=SC2034  # Unused variables left for readability
  for LOOP_COUNTER in $(seq 0 45); do
      RESULTS=$(ssh_ready "${1}")
      if [[ ${RESULTS} == 1 ]]; then
          echo "SSH is ready now! 🥳"
          break
      fi
      sleep 20
  done
  if [[ ${RESULTS} == 0 ]]; then
      clean_up "${1}"
      redprint "SSH failed to become ready 😢"
      exit 1
  fi
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

greenprint "💿 Creating a cloud-init ISO"
gen_iso

VIRT_LOG="$ARTIFACTS/oscap-sh-virt-install-console.log"
touch "$VIRT_LOG"
sudo chown qemu:qemu "$VIRT_LOG"

greenprint "📋 Install baseline image"
VIRSH_DOMAIN="${IMAGE_KEY}-baseline"
sudo virt-install --name="${VIRSH_DOMAIN}"\
                  --disk "${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                  --disk path="${CLOUD_INIT_PATH}",device=cdrom \
                  --ram 3072 \
                  --vcpus 2 \
                  --network network=integration,mac=34:49:22:B0:83:30 \
                  --os-variant "${OS_VARIANT}" \
                  --import \
                  --noautoconsole \
                  --nographics \
                  --wait=15 \
                  --noreboot \
                   --console pipe,source.path="$VIRT_LOG"

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
wait_for_ssh_up "${HTTP_GUEST_ADDRESS}"

greenprint "🔒 Running oscap scanner"
# oscap package has been installed on vm as
# oscap customization has been specified
# (the test has to be run as root)
sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${HTTP_GUEST_ADDRESS}" \
  sudo oscap xccdf eval \
  --results results.xml \
  --profile "${PROFILE}" \
  "${DATASTREAM}" || true # oscap returns exit code 2 for any failed rules

greenprint "📗 Checking oscap score"
BASELINE_SCORE=$(sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${HTTP_GUEST_ADDRESS}" \
  sudo cat results.xml \
  | xmlstarlet sel -N x="http://checklists.nist.gov/xccdf/1.2" -t -v "//x:score"
)
echo "Baseline score: ${BASELINE_SCORE}%"

# Clean up BASELINE VM
greenprint "🧹 Clean up VM"
clean_up

###############################
##
## Hardened Image
##
###############################

# Write a blueprint for hardened image.
# TODO: Remove firewalld rules from tailoring once https://github.com/ComplianceAsCode/content/issues/11275 is fixed
# COMPOSER-2076 is tracking this workaround
tee "${BLUEPRINT_FILE}" > /dev/null << EOF
name = "hardened"
description = "A hardened OpenSCAP image"
version = "0.0.1"
modules = []
groups = []

[customizations.openscap]
profile_id = "${PROFILE}"
datastream = "${DATASTREAM}"
[customizations.openscap.tailoring]
unselected = ["grub2_password", "firewalld_loopback_traffic_restricted", "firewalld_loopback_traffic_trusted"]

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

greenprint "💿 Creating a cloud-init ISO"
gen_iso

greenprint "📋 Install hardened image"
VIRSH_DOMAIN="${IMAGE_KEY}-hardened"
sudo virt-install --name="${VIRSH_DOMAIN}"\
                  --disk "${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                  --disk path="${CLOUD_INIT_PATH}",device=cdrom \
                  --ram 3072 \
                  --vcpus 2 \
                  --network network=integration,mac=34:49:22:B0:83:30 \
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
wait_for_ssh_up "${HTTP_GUEST_ADDRESS}"

greenprint "🔒 Running oscap scanner"
# oscap package has been installed on vm as
# oscap customization has been specified
# (the test has to be run as root)
sudo ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${HTTP_GUEST_ADDRESS}" \
  sudo oscap xccdf eval \
  --results results.xml \
  --profile "${PROFILE}_osbuild_tailoring" \
  --tailoring-file "/usr/share/xml/osbuild-openscap-data/tailoring.xml" \
  "${DATASTREAM}" || true # oscap returns exit code 2 for any failed rules

greenprint "📄 Saving results"
RESULTS=$(mktemp)
ssh -q "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${HTTP_GUEST_ADDRESS}" sudo cat results.xml > "$RESULTS"

greenprint "📗 Checking oscap score"
HARDENED_SCORE=$(cat "$RESULTS" | xmlstarlet sel -N x="http://checklists.nist.gov/xccdf/1.2" -t -v "//x:score")
echo "Hardened score: ${HARDENED_SCORE}%"

greenprint "📗 Checking for failed rules"
SEVERITY=$(cat "$RESULTS" \
  | xmlstarlet sel -N x="http://checklists.nist.gov/xccdf/1.2" -t -v "//x:rule-result[@severity='high']" \
  | grep -c "fail" || true # empty grep returns non-zero exit code
)
echo "Severity count: ${SEVERITY}"

# Clean up HARDENED VM
greenprint "🧹 Clean up VM"
clean_up

# Remomve tmp dir.
sudo rm -rf "${TEMPDIR}"

greenprint "🎏 Checking for test result"
echo "Baseline score: ${BASELINE_SCORE}%"
echo "Hardened score: ${HARDENED_SCORE}%"

# compare floating point numbers
if python3 -c "exit(${HARDENED_SCORE} > ${BASELINE_SCORE})"; then
  redprint "❌ Failed"
  echo "Hardened image did not improve baseline score"
  exit 1
fi

if [[ ${SEVERITY} -gt 0 ]]; then
  redprint "❌ Failed"
  echo "One or more oscap rules with high severity failed"
  exit 1
fi

greenprint "💚 Success"
