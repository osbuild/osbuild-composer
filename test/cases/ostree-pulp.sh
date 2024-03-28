#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)
source /usr/libexec/tests/osbuild-composer/ostree-common-functions.sh

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Get compose url if it's running on unsubscried RHEL
if [[ ${ID} == "rhel" ]] && ! sudo subscription-manager status; then
    source /usr/libexec/osbuild-composer-test/define-compose-url.sh
fi

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

# Set os-variant and boot location used by virt-install.
case "${ID}-${VERSION_ID}" in
    "rhel-9."*)
        IMAGE_TYPE=edge-commit
        OSTREE_REF="rhel/9/${ARCH}/edge"
        OS_VARIANT="rhel9-unknown"
        # Use a stable installer image unless it's the nightly pipeline
        BOOT_LOCATION="http://download.devel.redhat.com/released/rhel-9/RHEL-9/9.2.0/BaseOS/x86_64/os/"
        if [ "${NIGHTLY:=false}" == "true" ]; then
            BOOT_LOCATION="${COMPOSE_URL:-}/compose/BaseOS/x86_64/os/"
        fi
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac

common_init

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
export COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
export COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json
PROD_REPO_URL=http://192.168.100.1/repo
PROD_REPO=/var/www/html/repo

# SSH setup.
export SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
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

##################################################
##
## Upload ostree commit to pulp test
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
            quay.io/pulp/pulp:nightly

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

greenprint "📄 pulp config file:"
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
EOF

greenprint "📄 ostree blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve ostree

# Build commit image
build_image -b ostree -t "$IMAGE_TYPE" -k test -c "$PULP_CONFIG_FILE"

# Start httpd to serve ostree repo.
greenprint "🚀 Starting httpd daemon"
# osbuild-composer-tests have mod_ssl as a dependency. The package installs
# an example configuration which automatically enabled httpd on port 443, but
# that one is already in use. Remove the default configuration as it is useless
# anyway.
sudo rm -f /etc/httpd/conf.d/ssl.conf
sudo systemctl start httpd

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

# Get the boot.iso from BOOT_LOCATION
curl -O "$BOOT_LOCATION"images/boot.iso
sudo mv boot.iso /var/lib/libvirt/images
LOCAL_BOOT_LOCATION="/var/lib/libvirt/images/boot.iso"

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
                   --location ${LOCAL_BOOT_LOCATION} \
                   --nographics \
                   --noautoconsole \
                   --wait=-1 \
                   --noreboot

# Start VM.
greenprint "Start VM"
sudo virsh start "${IMAGE_KEY}"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _LOOP_COUNTER in $(seq 0 30); do
    RESULTS="$(wait_for_ssh_up $GUEST_ADDRESS)"
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

check_result

clean_up

exit 0
