#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

function installClientVSphere() {
    if ! hash govc; then
        ARCH="$(uname -m)"
        if [ "$ARCH" = "aarch64" ]; then
            ARCH="arm64"
        fi
        greenprint "Installing govc"
        pushd "${WORKDIR}" || exit 1
        curl -Ls --retry 5 --output govc.tar.gz \
            "https://github.com/vmware/govmomi/releases/download/v0.29.0/govc_Linux_$ARCH.tar.gz"
        tar -xvf govc.tar.gz
        GOVC_CMD="${WORKDIR}/govc"
        chmod +x "${GOVC_CMD}"
        popd || exit 1
    else
        echo "Using pre-installed 'govc' from the system"
        GOVC_CMD="govc"
    fi

    $GOVC_CMD version
}

function checkEnvVSphere() {
    printenv GOVMOMI_USERNAME GOVMOMI_PASSWORD GOVMOMI_URL GOVMOMI_CLUSTER GOVMOMI_DATACENTER GOVMOMI_DATASTORE GOVMOMI_FOLDER GOVMOMI_NETWORK  > /dev/null
}

# Create a cloud-int user-data file
#
# Returns:
#   - path to the user-data file
#
# Arguments:
#   $1 - default username
#   $2 - path to the SSH public key to set as authorized for the user
function createCIUserdata() {
    local _user="$1"
    local _ssh_pubkey_path="$2"

    local _ci_userdata_dir
    _ci_userdata_dir="$(mktemp -d -p "${WORKDIR}")"
    local _ci_userdata_path="${_ci_userdata_dir}/user-data"

    cat > "${_ci_userdata_path}" <<EOF
#cloud-config
users:
    - name: "${_user}"
      sudo: "ALL=(ALL) NOPASSWD:ALL"
      ssh_authorized_keys:
          - "$(cat "${_ssh_pubkey_path}")"
EOF

    echo "${_ci_userdata_path}"
}

# Create a cloud-int meta-data file
#
# Returns:
#   - path to the meta-data file
#
# Arguments:
#   $1 - VM name
function createCIMetadata() {
    local _vm_name="$1"

    local _ci_metadata_dir
    _ci_metadata_dir="$(mktemp -d -p "${WORKDIR}")"
    local _ci_metadata_path="${_ci_metadata_dir}/meta-data"

    cat > "${_ci_metadata_path}" <<EOF
instance-id: ${_vm_name}
local-hostname: ${_vm_name}
EOF

    echo "${_ci_metadata_path}"
}

# Create an ISO with the provided cloud-init user-data file
#
# Returns:
#   - path to the created ISO file
#
# Arguments:
#   $1 - path to the cloud-init user-data file
#   $2 - path to the cloud-init meta-data file
function createCIUserdataISO() {
    local _ci_userdata_path="$1"
    local _ci_metadata_path="$2"

    local _iso_path
    _iso_path="$(mktemp -p "${WORKDIR}" --suffix .iso)"
    mkisofs \
        -input-charset "utf-8" \
        -output "${_iso_path}" \
        -volid "cidata" \
        -joliet \
        -rock \
        -quiet \
        -graft-points \
        "${_ci_userdata_path}" \
        "${_ci_metadata_path}"

    echo "${_iso_path}"
}

# Verify VMDK image in VSphere
function verifyInVSphere() {
    local _filename="$1"
    greenprint "Verifying VMDK image: ${_filename}"

    # Create SSH keys to use
    local _vsphere_ssh_key="${WORKDIR}/vsphere_ssh_key"
    ssh-keygen -t rsa-sha2-512 -f "${_vsphere_ssh_key}" -C "${SSH_USER}" -N ""

    VSPHERE_VM_NAME="osbuild-composer-vm-${TEST_ID}"

    # create cloud-init ISO with the configuration
    local _ci_userdata_path
    _ci_userdata_path="$(createCIUserdata "${SSH_USER}" "${_vsphere_ssh_key}.pub")"
    local _ci_metadata_path
    _ci_metadata_path="$(createCIMetadata "${VSPHERE_VM_NAME}")"
    greenprint "💿 Creating cloud-init user-data ISO"
    local _ci_iso_path
    _ci_iso_path="$(createCIUserdataISO "${_ci_userdata_path}" "${_ci_metadata_path}")"

    VSPHERE_IMAGE_NAME="${VSPHERE_VM_NAME}.vmdk"
    mv "${_filename}" "${WORKDIR}/${VSPHERE_IMAGE_NAME}"

    # import the built VMDK image to VSphere
    # import.vmdk seems to be creating the provided directory and
    # if one with this name exists, it appends "_<number>" to the name
    greenprint "💿 ⬆️ Importing the converted VMDK image to VSphere"
    $GOVC_CMD import.vmdk \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -pool="${GOVMOMI_CLUSTER}"/Resources \
        -ds="${GOVMOMI_DATASTORE}" \
        "${WORKDIR}/${VSPHERE_IMAGE_NAME}" \
        "${VSPHERE_VM_NAME}"

    # create the VM, but don't start it
    greenprint "🖥️ Creating VM in VSphere"
    $GOVC_CMD vm.create \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -pool="${GOVMOMI_CLUSTER}"/Resources \
        -ds="${GOVMOMI_DATASTORE}" \
        -folder="${GOVMOMI_FOLDER}" \
        -net="${GOVMOMI_NETWORK}" \
        -net.adapter=vmxnet3 \
        -m=4096 -c=2 -g=rhel8_64Guest -on=true -firmware=efi \
        -disk="${VSPHERE_VM_NAME}/${VSPHERE_IMAGE_NAME}" \
        -disk.controller=scsi \
        -on=false \
        "${VSPHERE_VM_NAME}"

    # tagging vm as testing object
    $GOVC_CMD tags.attach \
        -u "${GOVMOMI_USERNAME}":"${GOVMOMI_PASSWORD}"@"${GOVMOMI_URL}" \
        -k=true \
        -c "osbuild-composer testing" gitlab-ci-test \
        "/${GOVMOMI_DATACENTER}/vm/${GOVMOMI_FOLDER}/${VSPHERE_VM_NAME}"

    # upload ISO, create CDROM device and insert the ISO in it
    greenprint "💿 ⬆️ Uploading the cloud-init user-data ISO to VSphere"
    VSPHERE_CIDATA_ISO_PATH="${VSPHERE_VM_NAME}/cidata.iso"
    $GOVC_CMD datastore.upload \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        "${_ci_iso_path}" \
        "${VSPHERE_CIDATA_ISO_PATH}"

    local _cdrom_device
    greenprint "🖥️ + 💿 Adding a CD-ROM device to the VM"
    _cdrom_device="$($GOVC_CMD device.cdrom.add \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -vm "${VSPHERE_VM_NAME}")"

    greenprint "💿 Inserting the cloud-init ISO into the CD-ROM device"
    $GOVC_CMD device.cdrom.insert \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        -vm "${VSPHERE_VM_NAME}" \
        -device "${_cdrom_device}" \
        "${VSPHERE_CIDATA_ISO_PATH}"

    # start the VM
    greenprint "🔌 Powering up the VSphere VM"
    $GOVC_CMD vm.power \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -on "${VSPHERE_VM_NAME}"

    HOST=$($GOVC_CMD vm.ip \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -v4=true \
        -dc="${GOVMOMI_DATACENTER}" \
        "${VSPHERE_VM_NAME}")
    greenprint "⏱ Waiting for the VSphere VM to respond to ssh"
    _instanceWaitSSH "${HOST}"

    _ssh="ssh -oStrictHostKeyChecking=no -i ${_vsphere_ssh_key} $SSH_USER@$HOST"
    _instanceCheck "${_ssh}"

    greenprint "✅ Successfully verified VSphere image with cloud-init"
}

function cleanupVSphere() {
    # since this function can be called at any time, ensure that we don't expand unbound variables
    GOVC_CMD="${GOVC_CMD:-}"
    VSPHERE_VM_NAME="${VSPHERE_VM_NAME:-}"
    VSPHERE_CIDATA_ISO_PATH="${VSPHERE_CIDATA_ISO_PATH:-}"

    greenprint "🧹 Cleaning up the VSphere VM"
    $GOVC_CMD vm.destroy \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        "${VSPHERE_VM_NAME}"

    greenprint "🧹 Cleaning up the VSphere Datastore"
    $GOVC_CMD datastore.rm \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        -f \
        "${VSPHERE_CIDATA_ISO_PATH}"

    $GOVC_CMD datastore.rm \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVMOMI_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        -f \
        "${VSPHERE_VM_NAME}"
}
