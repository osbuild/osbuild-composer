#!/bin/bash

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

BRANCH_NAME="${CI_COMMIT_BRANCH:-local}"
BUILD_ID="${CI_JOB_ID:-$(uuidgen)}"

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

TEMPDIR=$(mktemp -d)
function cleanup() {
    greenprint "== Script execution stopped or finished - Cleaning up =="

    # kill dangling journalctl processes to prevent GitLab CI from hanging
    sudo pkill journalctl || echo "Nothing killed"

    # since this function can be called at any time, ensure that we don't expand unbound variables
    OC_CLI="${OC_CLI:-}"
    VM_NAME="${VM_NAME:-}"
    PVC_NAME="${PVC_NAME:-}"

    [[ "$OC_CLI" && "$VM_NAME" ]] && $OC_CLI delete vm "$VM_NAME"
    [[ "$OC_CLI" && "$PVC_NAME" ]] && $OC_CLI delete pvc "$PVC_NAME"

    sudo rm -rf "$TEMPDIR"
}
trap cleanup EXIT

ARCH=$(uname -m)
# OpenShift doesn't like upper-case, dots and underscores
TEST_ID=$(echo "$DISTRO_CODE-$ARCH-$BRANCH_NAME-$BUILD_ID" | tr "[:upper:]" "[:lower:]" | tr "_." "-")
IMAGE_KEY=image-${TEST_ID}

ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

# Set up temporary files.
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json
SSH_KEY="${TEMPDIR}/id_ssh"
ssh-keygen -t rsa-sha2-512 -f "$SSH_KEY" -N ""
SSH_PUB_KEY=$(cat "$SSH_KEY.pub")

# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-openshift-virtualization.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-openshift-virtualization.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "bash"
description = "A base system with bash"
version = "0.0.1"

[[packages]]
name = "bash"

[[packages]]
name = "cloud-init"

[customizations.services]
enabled = ["sshd", "cloud-init", "cloud-init-local", "cloud-config", "cloud-final"]

[[customizations.user]]
name = "admin"
description = "admin"
key = "$SSH_PUB_KEY"
home = "/home/admin/"
shell = "/usr/bin/bash"
groups = ["users", "wheel"]
EOF

# Prepare the blueprint for the compose.
greenprint "📋 Preparing blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve bash

# Get worker unit file so we can watch the journal.
WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
WORKER_JOURNAL_PID=$!

# Start the compose and upload to OpenShift
greenprint "🚀 Starting compose"
sudo composer-cli --json compose start bash qcow2 | tee "$COMPOSE_START"
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
    sleep 30
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

greenprint "📥 Downloading the image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
IMAGE_FILENAME="${COMPOSE_ID}-disk.qcow2"
# allow anyone to read b/c this file is owned by root
sudo chmod a+r "${IMAGE_FILENAME}"

# Delete the compose so we don't run out of disk space
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null


# install the OpenShift cli & virtctl binary
sudo dnf -y install wget
# https://docs.openshift.com/container-platform/4.13/cli_reference/openshift_cli/getting-started-cli.html
wget --no-check-certificate https://downloads-openshift-console.apps.prod-scale-spoke1-aws-us-east-1.itup.redhat.com/amd64/linux/oc.tar --directory-prefix "$TEMPDIR"
# https://docs.openshift.com/container-platform/4.13/virt/virt-using-the-cli-tools.html
wget --no-check-certificate https://hyperconverged-cluster-cli-download-openshift-cnv.apps.prod-scale-spoke1-aws-us-east-1.itup.redhat.com/amd64/linux/virtctl.tar.gz --directory-prefix "$TEMPDIR"
pushd "$TEMPDIR"
tar -xvf oc.tar
tar -xzvf virtctl.tar.gz
popd
OC_CLI="$TEMPDIR/oc"
VIRTCTL="$TEMPDIR/virtctl"
chmod a+x "$OC_CLI"
chmod a+x "$VIRTCTL"


# Authenticate via the gitlab-ci service account
$OC_CLI login --token="$ITUP_OPENSHIFT_TOKEN" --server=https://api.prod-scale-spoke1-aws-us-east-1.prod-scale-mgmthub1-aws-us-east-1.itup.redhat.com/ --insecure-skip-tls-verify=true
$OC_CLI whoami

OPENSHIFT_PROJECT="image-builder--runtime-int"
$OC_CLI project $OPENSHIFT_PROJECT

# Note: rh-restricted-nfs is the default StorageClass, see
# https://console-openshift-console.apps.prod-scale-spoke1-aws-us-east-1.itup.redhat.com/k8s/cluster/storage.k8s.io~v1~StorageClass
STORAGE_CLASS="rh-restricted-nfs"

# import the image into a data volume; total quota on the namespace seems to be 500 GiB
PVC_NAME="image-builder-data-volume-$TEST_ID"
"$VIRTCTL" image-upload --insecure dv "$PVC_NAME" --size=10Gi --storage-class="${STORAGE_CLASS}" --image-path="${IMAGE_FILENAME}"
# Note: --size=10Gi corresponds to the size of the filesystem inside the image, not the actual size of the qcow2 file

PVC_VOLUME_ID=$($OC_CLI get pvc "$PVC_NAME" -o json | jq -r ".spec.volumeName")


VM_NAME="image-builder-vm-$TEST_ID"
VM_YAML_FILE=${TEMPDIR}/vm.yaml

tee "$VM_YAML_FILE" >/dev/null <<EOF
apiVersion: kubevirt.io/v1
kind: VirtualMachine
metadata:
  name: $VM_NAME
  namespace: image-builder--runtime-int
spec:
  dataVolumeTemplates:
    - apiVersion: cdi.kubevirt.io/v1alpha1
      kind: DataVolume
      metadata:
        name: $PVC_NAME
      spec:
        pvc:
          accessModes:
            - ReadWriteOnce
          resources:
            requests:
              storage: 15G
          storageClassName: $STORAGE_CLASS
          volumeName: $PVC_VOLUME_ID
          volumeMode: Filesystem
        source:
          pvc:
            name: $PVC_NAME
            namespace: $OPENSHIFT_PROJECT
  instancetype:
    name: u1.small
  running: true
  template:
    metadata:
      labels:
        network.kubevirt.io/headlessService: headless
    spec:
      domain:
        devices:
          autoattachPodInterface: false
          disks: []
          interfaces:
            - masquerade: {}
              name: default
      hostname: $VM_NAME
      networks:
        - name: default
          pod: {}
      subdomain: headless
      volumes:
        - dataVolume:
            name: $PVC_NAME
          name: disk-0
        - cloudInitNoCloud:
            userData: |
              #cloud-config
              ssh_pwauth: True
              chpasswd:
                list: |
                   root:password
                expire: False
              hostname: $VM_NAME
          name: cloudinitdisk
EOF

# deploy VM using cloned PVC
# https://www.redhat.com/en/blog/getting-started-with-openshift-virtualization
$OC_CLI create -f "$VM_YAML_FILE"

# dump VM info
$OC_CLI describe vm "$VM_NAME"

# Wait for VM to become running
greenprint "⏱ Waiting for $VM_NAME to become running"
while true; do
    VM_STATUS=$($OC_CLI get vm "$VM_NAME" -o json | jq -r ".status.printableStatus")
    if [[ $VM_STATUS == Running ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done


# ssh into VM and check if it is running
greenprint "🛃 Checking that $VM_NAME is running"
set +e
for LOOP_COUNTER in {0..10}; do
    STATUS=$($VIRTCTL --namespace "$OPENSHIFT_PROJECT" -i "$SSH_KEY" --local-ssh-opts="-o StrictHostKeyChecking=no" ssh admin@"$VM_NAME" --command 'systemctl --wait is-system-running')

    if [[ $STATUS == running || $STATUS == degraded ]]; then
        greenprint "💚 Success"
        exit 0
    fi

    greenprint "... retrying $LOOP_COUNTER"
    sleep 10
done
set -e

redprint "❌ Failed after $LOOP_COUNTER retries"
exit 1
