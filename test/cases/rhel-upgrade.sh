#!/bin/bash
set -uxo pipefail

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"


# Wait for VM to start
function wait_for_vm {
    INSTANCE_ADDRESS=192.168.100.50
    SSH_OPTIONS=(-o StrictHostKeyChecking=no -o ConnectTimeout=5)
    NUM_LOOPS=60
    for _ in $(seq 0 "$NUM_LOOPS"); do
        if eval 'sudo ssh "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" root@"$INSTANCE_ADDRESS" exit'; then
            echo "Test VM is up."
            break
        else
            echo "Test VM is not ready yet."
        fi
        sleep 30
    done
}

# Start libvirtd and test it.
greenprint "🚀 Starting libvirt daemon"
sudo systemctl start libvirtd
sudo virsh list --all > /dev/null

# define custom network for libvirt
if ! sudo virsh net-info integration > /dev/null 2>&1; then
    sudo virsh net-define /usr/share/tests/osbuild-composer/rhel-upgrade/integration.xml
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

# Prepare ssh key for the test VM
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY_PUB=${SSH_DATA_DIR}/id_rsa.pub
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_KS="sshkey --username root \"$(cat "$SSH_KEY_PUB")\""

# Set repo urls for the VM where upgrade happens
case "${ID}-${VERSION_ID}" in
    rhel-9*)
        VIRT_BASEOS_REPO_URL="http://download.devel.redhat.com/rhel-8/nightly/RHEL-8/latest-RHEL-8.10.0/compose/BaseOS/x86_64/os/"
        VIRT_APPSTREAM_REPO_URL="http://download.devel.redhat.com/rhel-8/nightly/RHEL-8/latest-RHEL-8.10.0/compose/AppStream/x86_64/os/"
        ;;
    rhel-10*)
        VIRT_BASEOS_REPO_URL="http://download.devel.redhat.com/rhel-9/nightly/RHEL-9/latest-RHEL-9.5.0/compose/BaseOS/x86_64/os/"
        VIRT_APPSTREAM_REPO_URL="http://download.devel.redhat.com/rhel-9/nightly/RHEL-9/latest-RHEL-9.5.0/compose/AppStream/x86_64/os/"
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac
# Using the latest rhel-X nightly, see the discussion here:
# https://redhat-internal.slack.com/archives/C04JP91FB8X/p1700820808857539
sudo tee ks.cfg > /dev/null << EOF
text --non-interactive
method url
keyboard --vckeymap=us --xlayouts='us'
lang en_US.UTF-8
rootpw redhat
${SSH_KEY_KS}
timezone Europe/Prague --isUtc
reboot

# Disk partitioning information
ignoredisk --only-use=vda
autopart --type=lvm
clearpart --all --initlabel --drives=vda

repo --name baseos --baseurl=${VIRT_BASEOS_REPO_URL} --install
repo --name appstream --baseurl=${VIRT_APPSTREAM_REPO_URL} --install

%packages 
@core
%end
EOF

# serve the kickstart over http and set necessary selinux context
sudo mv ks.cfg /var/www/html/
sudo semanage fcontext -a -t httpd_sys_content_t /var/www/html/ks.cfg
sudo restorecon /var/www/html/ks.cfg
sudo systemctl start httpd

# allow port 80 on the firewall for libvirt if it's running
if sudo systemctl status firewalld; then
    sudo firewall-cmd --zone=libvirt --add-port=80/tcp
fi

# prepare file for watching the VM console
TEMPFILE=$(mktemp)
sudo chown qemu:qemu "$TEMPFILE"

# watch the installation and log to a file
sudo tail -f "$TEMPFILE" | sudo tee install_console.log > /dev/null &
CONSOLE_PID=$!

# Install the test VM
sudo virt-install --name rhel-test \
                  --memory 3072 \
                  --vcpus 2 \
                  --disk size=20 \
                  --location "$VIRT_BASEOS_REPO_URL" \
                  --network network=integration,mac=34:49:22:B0:83:30 \
                  --console pipe,source.path="$TEMPFILE" \
                  --noautoconsole \
                  --graphics none \
                  --wait -1 \
                  --extra-args 'inst.ks=http://192.168.100.1:80/ks.cfg' \
                  --extra-args 'console=ttyS0'

# wait for VM to start and kill console logging
wait_for_vm
sudo pkill -P "$CONSOLE_PID"

# copy over next phases of the test and run the first one
sudo scp "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" /usr/share/tests/osbuild-composer/rhel-upgrade/*.sh root@"$INSTANCE_ADDRESS":
sudo scp "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" /usr/libexec/tests/osbuild-composer/shared_lib.sh root@"$INSTANCE_ADDRESS":
sudo scp "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" /usr/libexec/osbuild-composer-test/define-compose-url.sh root@"$INSTANCE_ADDRESS":
# Put comment in sshd_config to keep root login after upgrade
sudo ssh "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" root@"$INSTANCE_ADDRESS" 'sed -i "s/PermitRootLogin yes/PermitRootLogin yes #for sure/" /etc/ssh/sshd_config'
set +e
sudo ssh "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" root@"$INSTANCE_ADDRESS" 'source /root/upgrade_prepare.sh'
sudo scp "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" root@"$INSTANCE_ADDRESS":/var/log/leapp/leapp-upgrade.log "$ARTIFACTS"
sudo scp "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" root@"$INSTANCE_ADDRESS":/var/log/leapp/leapp-report.txt "$ARTIFACTS"
set -e

# watch and log the console during upgrade
sudo tail -f "$TEMPFILE" | sudo tee upgrade_console.log > /dev/null &
CONSOLE_PID=$!

# wait for VM to reboot and kill console logging
wait_for_vm
sudo pkill -P "$CONSOLE_PID"

# run second phase of the test
set +e
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" root@"$INSTANCE_ADDRESS" 'source /root/upgrade_verify.sh'
RESULT="$?"
set -e

# copy over osbuild-composer logs
sudo scp "${SSH_OPTIONS[@]}" -q -i "${SSH_KEY}" root@"$INSTANCE_ADDRESS":logs/* "$ARTIFACTS"

if [[ "$RESULT" == 0 ]]; then
  greenprint "💚 Success"
else
  redprint "❌ Failed"
  exit 1
fi

exit 0
