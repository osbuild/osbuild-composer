#!/bin/bash
set -euo pipefail

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Get compose url if it's running on unsubscried RHEL
if [[ ${ID} == "rhel" ]] && ! sudo subscription-manager status; then
    source /usr/libexec/osbuild-composer-test/define-compose-url.sh
fi

# Set up variables.
FIREWALL_FEATURE="false"
SYSROOT_RO="false"
SKIP_PULP_TEST="true"

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Set os-variant and boot location used by virt-install.
case "${ID}-${VERSION_ID}" in
    fedora-*)
        IMAGE_TYPE=iot-commit
        OSTREE_REF="fedora/${VERSION_ID}/${ARCH}/iot"
        OS_VARIANT="fedora-unknown"
        USER_IN_COMMIT="false"
        BOOT_LOCATION="https://mirrors.kernel.org/fedora/releases/${VERSION_ID}/Everything/x86_64/os/"
        EMBEDED_CONTAINER="false"
        FIREWALL_FEATURE="false"
        DIRS_FILES_CUSTOMIZATION="true"
        SYSROOT_RO="true"
        ;;
    "rhel-8.4")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/8/${ARCH}/edge"
        OS_VARIANT="rhel8.4"
        USER_IN_COMMIT="true"
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-8/RHEL-8/8.4.0/BaseOS/x86_64/os/"
        EMBEDED_CONTAINER="false"
        FIREWALL_FEATURE="false"
        DIRS_FILES_CUSTOMIZATION="false"
        ;;
    "rhel-8.7")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/8/${ARCH}/edge"
        OS_VARIANT="rhel8-unknown"
        USER_IN_COMMIT="true"
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-8/RHEL-8/8.7.0/BaseOS/x86_64/os/"
        EMBEDED_CONTAINER="false"
        FIREWALL_FEATURE="false"
        DIRS_FILES_CUSTOMIZATION="false"
        ;;
    "rhel-8.8")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/8/${ARCH}/edge"
        OS_VARIANT="rhel8-unknown"
        USER_IN_COMMIT="true"
        EMBEDED_CONTAINER="true"
        FIREWALL_FEATURE="true"
        DIRS_FILES_CUSTOMIZATION="true"

        # Use a stable installer image unless it's the nightly pipeline
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-8/RHEL-8/8.8.0/BaseOS/x86_64/os/"
        if [ "${NIGHTLY:=false}" == "true" ]; then
            BOOT_LOCATION="${COMPOSE_URL:-}/compose/BaseOS/x86_64/os/"
        fi
        ;;
    "rhel-8.9")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/8/${ARCH}/edge"
        OS_VARIANT="rhel8-unknown"
        USER_IN_COMMIT="true"
        EMBEDED_CONTAINER="true"
        FIREWALL_FEATURE="true"
        DIRS_FILES_CUSTOMIZATION="true"

        # Use a stable installer image unless it's the nightly pipeline
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-8/RHEL-8/8.8.0/BaseOS/x86_64/os/"
        if [ "${NIGHTLY:=false}" == "true" ]; then
            BOOT_LOCATION="${COMPOSE_URL:-}/compose/BaseOS/x86_64/os/"
        fi
        ;;
    "rhel-9.1")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9.1"
        USER_IN_COMMIT="true"
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-9/RHEL-9/9.1.0/BaseOS/x86_64/os/"
        EMBEDED_CONTAINER="false"
        FIREWALL_FEATURE="false"
        DIRS_FILES_CUSTOMIZATION="false"
        ;;
    "rhel-9.2")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9-unknown"
        USER_IN_COMMIT="true"
        EMBEDED_CONTAINER="true"
        FIREWALL_FEATURE="true"
        DIRS_FILES_CUSTOMIZATION="true"
        SYSROOT_RO="true"

        # Use a stable installer image unless it's the nightly pipeline
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-9/RHEL-9/9.2.0/BaseOS/x86_64/os/"
        if [ "${NIGHTLY:=false}" == "true" ]; then
            BOOT_LOCATION="${COMPOSE_URL:-}/compose/BaseOS/x86_64/os/"
        fi
        ;;
    "rhel-9.3")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9-unknown"
        USER_IN_COMMIT="true"
        EMBEDED_CONTAINER="true"
        FIREWALL_FEATURE="true"
        DIRS_FILES_CUSTOMIZATION="true"
        SYSROOT_RO="true"
        SKIP_PULP_TEST="false"
        # Use a stable installer image unless it's the nightly pipeline
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-9/RHEL-9/9.2.0/BaseOS/x86_64/os/"
        if [ "${NIGHTLY:=false}" == "true" ]; then
            BOOT_LOCATION="${COMPOSE_URL:-}/compose/BaseOS/x86_64/os/"
        fi
        ;;
    "centos-8")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="centos/8/${ARCH}/edge"
        OS_VARIANT="centos8"
        USER_IN_COMMIT="true"
        BOOT_LOCATION="https://composes.centos.org/latest-CentOS-Stream-8/compose/BaseOS/x86_64/os/"
        EMBEDED_CONTAINER="true"
        FIREWALL_FEATURE="false"
        DIRS_FILES_CUSTOMIZATION="true"
        ;;
    "centos-9")
        IMAGE_TYPE=edge-commit
        OSTREE_REF="centos/9/${ARCH}/edge"
        OS_VARIANT="centos-stream9"
        USER_IN_COMMIT="true"
        BOOT_LOCATION="https://odcs.stream.centos.org/production/latest-CentOS-Stream/compose/BaseOS/x86_64/os/"
        EMBEDED_CONTAINER="true"
        FIREWALL_FEATURE="false"
        DIRS_FILES_CUSTOMIZATION="true"
        SYSROOT_RO="true"
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac

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
IMAGE_KEY="osbuild-composer-ostree-test-${TEST_UUID}"
GUEST_ADDRESS=192.168.100.50
SSH_USER="admin"
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
KS_FILE=${TEMPDIR}/ks.cfg
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json
FEDORA_IMAGE_DIGEST="sha256:4d76a7480ce1861c95975945633dc9d03807ffb45c64b664ef22e673798d414b"
FEDORA_LOCAL_NAME="localhost/fedora-minimal:v1"
PROD_REPO_URL=http://192.168.100.1/repo
PROD_REPO=/var/www/html/repo

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB="$(cat "${SSH_KEY}".pub)"

# Pulp setup
PULP_CONFIG_FILE="$(pwd)/pulp.toml"
PULP_SERVER="http://192.168.100.1:9090"
PULP_USERNAME="admin"
PULP_PASSWORD="foobar"
PULP_REPO="commit"
PULP_BASEPATH="commit"

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
    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!
    # Stop watching the worker journal when exiting.
    trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

    # Start the compose.
    greenprint "🚀 Starting compose"
    pulp_enabled=$1
    blueprint_name=$2
    image_type=$3
    if [ "$pulp_enabled" == "nopulp" ]; then
        # for nopulp first build
        if [ $# -eq 3 ]; then
            sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
        # for nopulp upgrade build
        else
            repo_url=$4
            parent_ref=$5
            sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" --parent "$parent_ref" --url "$repo_url" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
        fi
    else
        # for pulp first build
        if [ $# -eq 5 ]; then
            image_key=$4
            pulp_config=$5
            sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" "$blueprint_name" "$image_type" "$image_key" "$pulp_config" | tee "$COMPOSE_START"
        # for pulp upgrade build
        else
            image_key=$4
            pulp_config=$5
            repo_url=$6
            parent_ref=$7
            sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" --parent "$parent_ref" --url "$repo_url" "$blueprint_name" "$image_type" "$image_key" "$pulp_config" | tee "$COMPOSE_START"
        fi
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
        redprint "Something went wrong with the compose. 😢"
        exit 1
    fi
}

# Wait for the ssh server up to be.
wait_for_ssh_up () {
    SSH_STATUS=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${1}" '/bin/bash -c "echo -n READY"')
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
    # Remove extracted upgrade image-tar.
    sudo rm -rf "$UPGRADE_PATH"
    # Remove "remote" repo.
    sudo rm -rf "${HTTPD_PATH}"/{repo,compose.json}
    # Remomve tmp dir.
    sudo rm -rf "$TEMPDIR"
    # Stop httpd
    sudo systemctl disable httpd --now
}

# Test result checking
check_result () {
    greenprint "Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        redprint "❌ Failed"
        clean_up
        exit 1
    fi
}

##################################################
##
## ostree image/commit installation
##
##################################################

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "ostree"
description = "A base ostree image"
version = "0.0.1"
modules = []
groups = []

[[packages]]
name = "python3"
version = "*"

[[packages]]
name = "sssd"
version = "*"
EOF

# RHEL 8.5 and later support user configuration in blueprint for edge-commit image
if [[ "${USER_IN_COMMIT}" == "true" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[customizations.user]]
name = "${SSH_USER}"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/${SSH_USER}/"
groups = ["wheel"]
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

greenprint "📄 rebase blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve ostree

# Build installation image.
build_image nopulp ostree "$IMAGE_TYPE"

# Start httpd to serve ostree repo.
greenprint "🚀 Starting httpd daemon"
# osbuild-composer-tests have mod_ssl as a dependency. The package installs
# an example configuration which automatically enabled httpd on port 443, but
# that one is already in use. Remove the default configuration as it is useless
# anyway.
sudo rm -f /etc/httpd/conf.d/ssl.conf
sudo systemctl start httpd

# Download the image and extract tar into web server root folder.
greenprint "📥 Downloading and extracting the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-commit.tar"
HTTPD_PATH="/var/www/html"
sudo tar -xf "${IMAGE_FILENAME}" -C ${HTTPD_PATH}
sudo rm -f "$IMAGE_FILENAME"

# Clean compose and blueprints.
greenprint "Clean up osbuild-composer"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete ostree > /dev/null

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Create qcow2 file for virt install.
greenprint "Create qcow2 file for virt install"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

# Write kickstart file for ostree image installation.
greenprint "Generate kickstart file"
tee "$KS_FILE" > /dev/null << STOPHERE
text
lang en_US.UTF-8
keyboard us
timezone --utc Etc/UTC

selinux --enforcing
rootpw --lock --iscrypted locked
user --name=${SSH_USER} --groups=wheel --iscrypted --password=\$6\$1LgwKw9aOoAi/Zy9\$Pn3ErY1E8/yEanJ98evqKEW.DZp24HTuqXPJl6GYCm8uuobAmwxLv7rGCvTRZhxtcYdmC0.XnYRSR9Sh6de3p0
sshkey --username=${SSH_USER} "${SSH_KEY_PUB}"

bootloader --timeout=1 --append="net.ifnames=0 modprobe.blacklist=vc4"

network --bootproto=dhcp --device=link --activate --onboot=on

zerombr
clearpart --all --initlabel --disklabel=msdos
autopart --nohome --noswap --type=plain
ostreesetup --nogpg --osname=${IMAGE_TYPE} --remote=${IMAGE_TYPE} --url=${PROD_REPO_URL} --ref=${OSTREE_REF}
poweroff

%post --log=/var/log/anaconda/post-install.log --erroronfail

# no sudo password for SSH user
echo -e '${SSH_USER}\tALL=(ALL)\tNOPASSWD: ALL' >> /etc/sudoers

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

echo "Packages within this iot or edge image:"
echo "-----------------------------------------------------------------------"
rpm -qa | sort
echo "-----------------------------------------------------------------------"
# Note that running rpm recreates the rpm db files which aren't needed/wanted
rm -f /var/lib/rpm/__db*

echo "Zeroing out empty space."
# This forces the filesystem to reclaim space from deleted files
dd bs=1M if=/dev/zero of=/var/tmp/zeros || :
rm -f /var/tmp/zeros
echo "(Don't worry -- that out-of-space error was expected.)"

%end
STOPHERE

# RHEL 8.5 and later configures user in blueprint for edge-commit image
if [[ "${USER_IN_COMMIT}" == "true" ]]; then
    sudo sed -i '/^user\|^sshkey/d' "${KS_FILE}"
fi

# Get the boot.iso from BOOT_LOCATION
curl -O "$BOOT_LOCATION"images/boot.iso
sudo mv boot.iso /var/lib/libvirt/images
LOCAL_BOOT_LOCATION="/var/lib/libvirt/images/boot.iso"

# Workaround to fix issue https://github.com/osbuild/osbuild-composer/issues/3216
if [[ "${ID}" == "fedora" ]]; then
    location_arg="${LOCAL_BOOT_LOCATION}",initrd=images/pxeboot/initrd.img,kernel=images/pxeboot/vmlinuz
else
    location_arg="${LOCAL_BOOT_LOCATION}"
fi

# Install ostree image via anaconda.
greenprint "Install ostree image via anaconda"
sudo virt-install  --initrd-inject="${KS_FILE}" \
                   --extra-args="inst.ks=file:/ks.cfg console=ttyS0,115200" \
                   --name="${IMAGE_KEY}"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 3072 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-variant ${OS_VARIANT} \
                   --location ${location_arg} \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "Start VM"
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

# With new ostree-libs-2022.6-3, edge vm needs to reboot twice to make the /sysroot readonly
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
# Sleep 10 seconds here to make sure vm restarted already
sleep 10

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
## ostree image/commit upgrade
##
##################################################

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "upgrade"
description = "An upgrade ostree image"
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

# RHEL 8.5 and later support user configuration in blueprint for edge-commit image
if [[ "${USER_IN_COMMIT}" == "true" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[customizations.user]]
name = "${SSH_USER}"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/${SSH_USER}/"
groups = ["wheel"]
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

if [[ "${FIREWALL_FEATURE}" == "true" ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[customizations.firewall.zones]]
name = "trusted"
sources = ["192.168.100.51"]
[[customizations.firewall.zones]]
name = "work"
sources = ["192.168.100.52"]
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

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve upgrade

greenprint "🕹 Get ostree installed commit value"
PARENT_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Build upgrade image.
build_image nopulp upgrade "$IMAGE_TYPE" "$PROD_REPO_URL" "$PARENT_HASH"

# Download the image and extract tar into web server root folder.
greenprint "📥 Downloading and extracting the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-commit.tar"
UPGRADE_PATH="$(pwd)/upgrade"
mkdir -p "$UPGRADE_PATH"
sudo tar -xf "$IMAGE_FILENAME" -C "$UPGRADE_PATH"
sudo rm -f "$IMAGE_FILENAME"

# Clean compose and blueprints.
greenprint "Clean up osbuild-composer again"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete upgrade > /dev/null

# Introduce new ostree commit into repo.
greenprint "Introduce new ostree commit into repo"
sudo ostree pull-local --repo "${HTTPD_PATH}/repo" "${UPGRADE_PATH}/repo" "$OSTREE_REF"
sudo ostree --repo="${HTTPD_PATH}/repo" static-delta generate "$OSTREE_REF"
sudo ostree summary --update --repo "${HTTPD_PATH}/repo"

# Ensure SELinux is happy with all objects files.
greenprint "👿 Running restorecon on web server root folder"
sudo restorecon -Rv "${HTTPD_PATH}/repo" > /dev/null

# Get ostree commit value.
greenprint "🕹 Get ostree upgrade commit value"
UPGRADE_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Upgrade image/commit.
greenprint "Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${GUEST_ADDRESS}" 'sudo rpm-ostree upgrade || { sudo rpm-ostree status; sudo journalctl -b -r -u rpm-ostreed; exit 1; }'
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
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
ansible_user=${SSH_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type=${IMAGE_TYPE} \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e embeded_container="${EMBEDED_CONTAINER}" \
    -e firewall_feature="${FIREWALL_FEATURE}" \
    -e test_custom_dirs_files="${DIRS_FILES_CUSTOMIZATION}" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

greenprint "🧹 Clean up VM"
if [[ $(sudo virsh domstate "${IMAGE_KEY}") == "running" ]]; then
    sudo virsh destroy "${IMAGE_KEY}"
fi
sudo virsh undefine "${IMAGE_KEY}" --nvram
sudo virsh vol-delete --pool images "$LIBVIRT_IMAGE_PATH"

# The following pulp test is only for rhel-9.3 and later
if [[ "${SKIP_PULP_TEST}" == "true" ]]; then
    greenprint "📋 Current OS does not support pulp, skip pulp test and exit"
    clean_up
    exit 0
fi

##################################################
##
## ostree upload commit to pulp
##
##################################################

# Setup pulp server
greenprint "📄 Setup pulp server with one container"
mkdir -p settings pulp_storage pgsql containers
echo "CONTENT_ORIGIN='http://$(hostname):8080'
ANSIBLE_API_HOSTNAME='http://$(hostname):8080'
ANSIBLE_CONTENT_HOSTNAME='http://$(hostname):8080/pulp/content'
CACHE_ENABLED=True" >> settings/settings.py
sudo podman run --detach \
            --publish 9090:80 \
            --name pulp \
            --volume "$(pwd)/settings":/etc/pulp:Z \
            --volume "$(pwd)/pulp_storage":/var/lib/pulp:Z \
            --volume "$(pwd)/pgsql":/var/lib/pgsql:Z \
            --volume "$(pwd)/containers":/var/lib/containers:Z \
            --device /dev/fuse \
            docker.io/pulp/pulp:latest

# Wait until pulp service is fully functional
sleep 120

# Rotate pulp admin password
greenprint "📄 Rotate pulp admin password"
/usr/bin/expect <<-EOF
spawn sudo podman exec -it pulp bash -c "pulpcore-manager reset-admin-password"
expect {
"*password" { send "${PULP_PASSWORD}\r"; exp_continue }
"*again" { send "${PULP_PASSWORD}\r" }
}
expect eof
EOF

# Write a pulp config file.
greenprint "📄 Prepare pulp config file"
tee "$PULP_CONFIG_FILE" > /dev/null << EOF
provider = "pulp.ostree"

[settings]
server_address = "${PULP_SERVER}"
repository = "${PULP_REPO}"
basepath = "${PULP_BASEPATH}"
username = "${PULP_USERNAME}"
password = "${PULP_PASSWORD}"
EOF

greenprint "📄 pulp config file"
cat "$PULP_CONFIG_FILE"

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "ostree"
description = "A base ostree image"
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
name = "${SSH_USER}"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/${SSH_USER}/"
groups = ["wheel"]

[[containers]]
source = "quay.io/fedora/fedora:latest"

[[containers]]
source = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal@${FEDORA_IMAGE_DIGEST}"
name = "${FEDORA_LOCAL_NAME}"

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

greenprint "📄 ostree blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve ostree

# Build commit image
build_image pulp ostree "$IMAGE_TYPE" test "$PULP_CONFIG_FILE"

# Workaround for bug https://github.com/osbuild/osbuild-composer/issues/3673
greenprint "Waiting for ostree commit import"
while ! curl -sf "${PULP_SERVER}/pulp/content/${PULP_BASEPATH}/refs/heads/${OSTREE_REF}"; do
    echo -n "."
    sleep 5
done

# Pull commit from pulp to local repo
greenprint "Pull commit from pulp to production repo"
sudo rm -fr "$PROD_REPO"
sudo mkdir -p "$PROD_REPO"
sudo ostree --repo="$PROD_REPO" init --mode=archive
sudo ostree --repo="$PROD_REPO" remote add --no-gpg-verify edge-pulp http://localhost:9090/pulp/content/${PULP_REPO}/
sudo ostree --repo="$PROD_REPO" pull --mirror edge-pulp "$OSTREE_REF"

# Clean compose and blueprints.
greenprint "Clean up osbuild-composer"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete ostree > /dev/null

# Ensure SELinux is happy with our new images.
greenprint "👿 Running restorecon on image directory"
sudo restorecon -Rv /var/lib/libvirt/images/

# Ensure SELinux is happy with all objects files.
greenprint "👿 Running restorecon on web server root folder"
sudo restorecon -Rv "${PROD_REPO}" > /dev/null

# Create qcow2 file for virt install.
greenprint "Create qcow2 file for virt install"
LIBVIRT_IMAGE_PATH=/var/lib/libvirt/images/${IMAGE_KEY}-pulp.qcow2
sudo qemu-img create -f qcow2 "${LIBVIRT_IMAGE_PATH}" 20G

# Write kickstart file for ostree image installation.
greenprint "Generate kickstart file"
tee "$KS_FILE" > /dev/null << STOPHERE
text
lang en_US.UTF-8
keyboard us
timezone --utc Etc/UTC

selinux --enforcing
rootpw --lock --iscrypted locked
user --name=${SSH_USER} --groups=wheel --iscrypted --password=\$6\$1LgwKw9aOoAi/Zy9\$Pn3ErY1E8/yEanJ98evqKEW.DZp24HTuqXPJl6GYCm8uuobAmwxLv7rGCvTRZhxtcYdmC0.XnYRSR9Sh6de3p0
sshkey --username=${SSH_USER} "${SSH_KEY_PUB}"

bootloader --timeout=1 --append="net.ifnames=0 modprobe.blacklist=vc4"

network --bootproto=dhcp --device=link --activate --onboot=on

zerombr
clearpart --all --initlabel --disklabel=msdos
autopart --nohome --noswap --type=plain
ostreesetup --nogpg --osname=${IMAGE_TYPE} --remote=${IMAGE_TYPE} --url=${PROD_REPO_URL} --ref=${OSTREE_REF}
poweroff

%post --log=/var/log/anaconda/post-install.log --erroronfail

# no sudo password for SSH user
echo -e '${SSH_USER}\tALL=(ALL)\tNOPASSWD: ALL' >> /etc/sudoers

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

echo "Packages within this iot or edge image:"
echo "-----------------------------------------------------------------------"
rpm -qa | sort
echo "-----------------------------------------------------------------------"
# Note that running rpm recreates the rpm db files which aren't needed/wanted
rm -f /var/lib/rpm/__db*

echo "Zeroing out empty space."
# This forces the filesystem to reclaim space from deleted files
dd bs=1M if=/dev/zero of=/var/tmp/zeros || :
rm -f /var/tmp/zeros
echo "(Don't worry -- that out-of-space error was expected.)"

%end
STOPHERE

sudo sed -i '/^user\|^sshkey/d' "${KS_FILE}"

# Install ostree image via anaconda.
greenprint "Install ostree image via anaconda"
sudo virt-install  --initrd-inject="${KS_FILE}" \
                   --extra-args="inst.ks=file:/ks.cfg console=ttyS0,115200" \
                   --name="${IMAGE_KEY}"\
                   --disk path="${LIBVIRT_IMAGE_PATH}",format=qcow2 \
                   --ram 3072 \
                   --vcpus 2 \
                   --network network=integration,mac=34:49:22:B0:83:30 \
                   --os-variant ${OS_VARIANT} \
                   --location ${location_arg} \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "Start VM"
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

# With new ostree-libs-2022.6-3, edge vm needs to reboot twice to make the /sysroot readonly
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
# Sleep 10 seconds here to make sure vm restarted already
sleep 10

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
## ostree commit upload to pulp upgrade
##
##################################################

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "upgrade"
description = "An upgrade ostree image"
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

[[customizations.user]]
name = "${SSH_USER}"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
key = "${SSH_KEY_PUB}"
home = "/home/${SSH_USER}/"
groups = ["wheel"]

[[containers]]
source = "quay.io/fedora/fedora:latest"

[[containers]]
source = "registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal@${FEDORA_IMAGE_DIGEST}"
name = "${FEDORA_LOCAL_NAME}"

[[customizations.firewall.zones]]
name = "trusted"
sources = ["192.168.100.51"]
[[customizations.firewall.zones]]
name = "work"
sources = ["192.168.100.52"]

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

greenprint "📄 upgrade blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve upgrade

greenprint "🕹 Get ostree installed commit value"
PARENT_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Build upgrade image.
build_image pulp upgrade "$IMAGE_TYPE" test "$PULP_CONFIG_FILE" "$PROD_REPO_URL" "$PARENT_HASH"

# Workaround for bug https://github.com/osbuild/osbuild-composer/issues/3673
# Cannot use curl to check because parent commit is already imported
greenprint "Waiting for ostree commit import"
sleep 180

# Clean compose and blueprints.
greenprint "Clean up osbuild-composer again"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete upgrade > /dev/null

# Pull content from pulp to local repo
greenprint "Pull upgrade commit from pulp to local repo"
sudo ostree --repo="$PROD_REPO" pull --mirror edge-pulp "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO" summary -u

# Ensure SELinux is happy with all objects files.
greenprint "👿 Running restorecon on web server root folder"
sudo restorecon -Rv "${PROD_REPO}" > /dev/null

# Get ostree commit value.
greenprint "🕹 Get ostree upgrade commit value"
UPGRADE_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Upgrade image/commit.
greenprint "Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${GUEST_ADDRESS}" 'sudo rpm-ostree upgrade || { sudo rpm-ostree status; sudo journalctl -b -r -u rpm-ostreed; exit 1; }'
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}@${GUEST_ADDRESS}" 'nohup sudo systemctl reboot &>/dev/null & exit'
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
ansible_user=${SSH_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type=${IMAGE_TYPE} \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e embeded_container="${EMBEDED_CONTAINER}" \
    -e firewall_feature="${FIREWALL_FEATURE}" \
    -e test_custom_dirs_files="${DIRS_FILES_CUSTOMIZATION}" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0
