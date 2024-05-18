#!/bin/bash
# A script that's full of common functions
# used throughout our ostree tests. Can be 
# sourced at beginning of those scripts.

function common_init() {
    trap cleanup_on_exit EXIT
    # Start libvirtd and test it.
    greenprint "🚀 Starting libvirt daemon"
    sudo systemctl start libvirtd
    sudo virsh list --all > /dev/null

    # Install and start firewalld
    greenprint "🔧 Install and start firewalld"
    sudo dnf install -y firewalld
    sudo systemctl enable --now firewalld
    # vsphere specific step
    command -v govc && { sudo firewall-cmd --permanent --zone=public --add-service=http; sudo firewall-cmd --reload; }

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
    <host mac='34:49:22:B0:83:30' name='vm-01' ip='192.168.100.50'/>
    <host mac='34:49:22:B0:83:31' name='vm-02' ip='192.168.100.51'/>
    </dhcp>
</ip>
<dnsmasq:options>
    <dnsmasq:option value='dhcp-vendorclass=set:efi-http,HTTPClient:Arch:00016'/>
    <dnsmasq:option value='dhcp-option-force=tag:efi-http,60,HTTPClient'/>
    <dnsmasq:option value='dhcp-boot=tag:efi-http,&quot;http://192.168.100.1/httpboot/EFI/BOOT/BOOTX64.EFI&quot;'/>
</dnsmasq:options>
</network>
EOF
    # Simplified installer needs a different dnsmasq.
    [ -d /etc/fdo/aio ] && sudo sed -i "s/<network>/<network xmlns:dnsmasq=\'http:\/\/libvirt.org\/schemas\/network\/dnsmasq\/1.0\'>/g" /tmp/integration.xml
    
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
}

function get_compose_log() {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

function get_compose_metadata() {
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

function clean_up() {
    set +u
    greenprint "🧼 Cleaning up"

    # Clear integration network
    sudo virsh net-destroy integration
    sudo virsh net-undefine integration
    
    # Remove tag from quay.io repo
    [ -n "$QUAY_REPO_URL" ] && skopeo delete --creds "${V2_QUAY_USERNAME}:${V2_QUAY_PASSWORD}" "docker://${QUAY_REPO_URL}:${QUAY_REPO_TAG}"
    
    { virsh list --all | grep "${IMAGE_KEY}"; } && {
        sudo virsh destroy "${IMAGE_KEY}";
        sudo virsh destroy "${IMAGE_KEY}-uefi";
        sudo virsh undefine "${IMAGE_KEY}" --nvram
        sudo virsh undefine "${IMAGE_KEY}-uefi" --nvram
    }
    
    # Remove any status containers if exist
    sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
    # Remove all images
    sudo podman rmi -f -a

    # Remove a bunch of directories
    [ -d "$LIBVIRT_IMAGE_PATH" ] && sudo rm -f "$LIBVIRT_IMAGE_PATH"
    [ -d "$PROD_REPO" ] && sudo rm -rf "$PROD_REPO"
    [ -d "$PROD_REPO_1" ] && sudo rm -rf "$PROD_REPO_1"
    [ -d "$PROD_REPO_2" ] && sudo rm -rf "$PROD_REPO_2"
    [ -d "$IGNITION_SERVER_FOLDER" ] && sudo rm -rf "$IGNITION_SERVER_FOLDER"
    
    # Remove "remote" repo.
    [ -d "$HTTPD_PATH" ] && sudo rm -rf "${HTTPD_PATH}"/{repo,compose.json}

    # Remomve tmp dir.
    [ -d "$TEMPDIR" ] && sudo rm -rf "$TEMPDIR"

    # Stop prod repo http service
    sudo systemctl disable --now httpd

    # vsphere specific cleanup
    [ -n "$DATACENTER_70" ] && govc vm.destroy -dc="${DATACENTER_70}" "${DC70_VSPHERE_VM_NAME}"
    set -u
}

function aws_clean_up() {
    greenprint "🧼 AWS specific cleaning up"
    # Deregister edge AMI image
    aws ec2 deregister-image \
        --image-id "${AMI_ID}"

    # Remove snapshot
    aws ec2 delete-snapshot \
        --snapshot-id "${SNAPSHOT_ID}"

    # Delete Key Pair
    aws ec2 delete-key-pair \
        --key-name "${AMI_KEY_NAME}"

    # Terminate running instance
    if [[ -v INSTANCE_ID ]]; then
        aws ec2 terminate-instances \
            --instance-ids "${INSTANCE_ID}"
        aws ec2 wait instance-terminated \
            --instance-ids "${INSTANCE_ID}"
    fi

    # Remove bucket content and bucket itself quietly
    aws s3 rb "${BUCKET_URL}" --force > /dev/null
}

function check_result() {
    greenprint "🎏 Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        redprint "❌ Failed"
        clean_up
        exit 1
    fi
}

function wait_for_ssh_up() {
    # Ignition user name is 'core'.
    [ -f "$TEMPDIR/config.ign" ] && SSH_USER="core" || SSH_USER="admin"
    SSH_STATUS=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" "${SSH_USER}"@"${1}" '/bin/bash -c "echo -n READY"')
    if [[ $SSH_STATUS == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

function build_image() {
    cmd=("--json" "compose" "start-ostree" "--ref" "$OSTREE_REF")
    local blue_print="" image_type="" key="" registry_config=""
    while [ "$#" -gt 0 ]; do
        case "$1" in
            -b|--blue-print)
                blue_print=$2
                shift 2
                ;;
            -t|--image-type)
                image_type=$2
                shift 2
                ;;
            -u|--url)
                cmd+=("--url" "$2")
                shift 2
                ;;
            -c|--registry-config)
                registry_config=$2
                shift 2
                ;;
            -k|--image-key)
                key=$2
                shift 2
                ;;
            -p|--parent)
                cmd+=("--parent" "$2")
                shift 2
                ;;
            *)
                redprint "Unknown argument: $1"
                return 1
                ;;
        esac
    done

    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!

    # Start the compose.
    greenprint "🚀 Starting compose"
    [ -n "$blue_print" ] && cmd+=("$blue_print")
    [ -n "$image_type" ] && cmd+=("$image_type")
    [ -n "$key" ] && cmd+=("$key")
    [ -n "$registry_config" ] && cmd+=("$registry_config")
    echo -e "The composer-cli command is\nsudo composer-cli ${cmd[*]} | tee $COMPOSE_START"
    sudo composer-cli "${cmd[@]}" | tee "$COMPOSE_START"
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

function cleanup_on_exit() {
    greenprint "== Script execution stopped or finished - Cleaning up =="
    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"
}