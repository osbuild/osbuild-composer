#!/usr/bin/bash

function createReqFileEdge() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "payload_repositories": [
      {
        "baseurl": "$PAYLOAD_REPO_URL"
      }
    ],
    "packages": [
      "postgresql",
      "dummy"
    ],
    "users":[
      {
        "name": "user1",
        "groups": ["wheel"],
        "key": "$(cat "${WORKDIR}/usertest.pub")"
      },
      {
        "name": "user2",
        "key": "$(cat "${WORKDIR}/usertest.pub")"
      }
    ]
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "ostree": {
      "ref": "${OSTREE_REF}"
    },
    "upload_options": {
      "region": "${AWS_REGION}"
    }
  }
}
EOF
}

function createReqFileGuest() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "payload_repositories": [
      {
        "baseurl": "$PAYLOAD_REPO_URL"
      }
    ],
    "packages": [
      "postgresql",
      "dummy"
    ]${SUBSCRIPTION_BLOCK},
    "users":[
      {
        "name": "user1",
        "groups": ["wheel"],
        "key": "$(cat "${WORKDIR}/usertest.pub")"
      },
      {
        "name": "user2",
        "key": "$(cat "${WORKDIR}/usertest.pub")"
      }
    ]
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "upload_options": {
      "region": "${AWS_REGION}"
    }
  }
}
EOF
}

# the VSphere test case does not create any additional users,
# since this is not supported by the service UI
function createReqFileVSphere() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "payload_repositories": [
      {
        "baseurl": "$PAYLOAD_REPO_URL"
      }
    ],
    "packages": [
      "postgresql",
      "dummy"
    ]${SUBSCRIPTION_BLOCK}
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "upload_options": {
      "region": "${AWS_REGION}"
    }
  }
}
EOF
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

# verify edge commit content
function verifyEdgeCommit() {
    filename="$1"
    greenprint "Verifying contents of ${filename}"

    # extract tarball and save file list to artifacts directroy
    local COMMIT_DIR
    COMMIT_DIR="${WORKDIR}/edge-commit"
    mkdir -p "${COMMIT_DIR}"
    tar xvf "${filename}" -C "${COMMIT_DIR}" > "${ARTIFACTS}/edge-commit-filelist.txt"

    # Verify that the commit contains the ref we defined in the request
    sudo dnf install -y ostree
    local COMMIT_REF
    COMMIT_REF=$(ostree refs --repo "${COMMIT_DIR}/repo")
    if [[ "${COMMIT_REF}" !=  "${OSTREE_REF}" ]]; then
        echo "Commit ref in archive does not match request 😠"
        exit 1
    fi

    local TAR_COMMIT_ID
    TAR_COMMIT_ID=$(ostree rev-parse --repo "${COMMIT_DIR}/repo" "${OSTREE_REF}")
    API_COMMIT_ID_V2=$(curl \
        --silent \
        --show-error \
        --cacert /etc/osbuild-composer/ca-crt.pem \
        --key /etc/osbuild-composer/client-key.pem \
        --cert /etc/osbuild-composer/client-crt.pem \
        https://localhost/api/image-builder-composer/v2/composes/"$COMPOSE_ID"/metadata | jq -r '.ostree_commit')

    if [[ "${API_COMMIT_ID_V2}" != "${TAR_COMMIT_ID}" ]]; then
        echo "Commit ID returned from API does not match Commit ID in archive 😠"
        exit 1
    fi

}

# Verify image blobs from s3
function verifyDisk() {
    filename="$1"
    greenprint "Verifying contents of ${filename}"

    infofile="${filename}-info.json"
    sudo /usr/libexec/osbuild-composer-test/image-info "${filename}" | tee "${infofile}" > /dev/null

    # save image info to artifacts
    cp -v "${infofile}" "${ARTIFACTS}/image-info.json"

    # check compose request users in passwd
    if ! jq .passwd "${infofile}" | grep -q "user1"; then
        greenprint "❌ user1 not found in passwd file"
        exit 1
    fi
    if ! jq .passwd "${infofile}" | grep -q "user2"; then
        greenprint "❌ user2 not found in passwd file"
        exit 1
    fi
    # check packages for postgresql
    if ! jq .packages "${infofile}" | grep -q "postgresql"; then
        greenprint "❌ postgresql not found in packages"
        exit 1
    fi

    greenprint "✅ ${filename} image info verified"
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
        -dc="${GOVC_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        "${WORKDIR}/${VSPHERE_IMAGE_NAME}" \
        "${VSPHERE_VM_NAME}"

    # create the VM, but don't start it
    greenprint "🖥️ Creating VM in VSphere"
    $GOVC_CMD vm.create \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        -pool="${GOVMOMI_CLUSTER}"/Resources \
        -ds="${GOVMOMI_DATASTORE}" \
        -folder="${GOVMOMI_FOLDER}" \
        -net="${GOVMOMI_NETWORK}" \
        -net.adapter=vmxnet3 \
        -m=4096 -c=2 -g=rhel8_64Guest -on=true -firmware=bios \
        -disk="${VSPHERE_VM_NAME}/${VSPHERE_IMAGE_NAME}" \
        -disk.controller=ide \
        -on=false \
        "${VSPHERE_VM_NAME}"

    # upload ISO, create CDROM device and insert the ISO in it
    greenprint "💿 ⬆️ Uploading the cloud-init user-data ISO to VSphere"
    VSPHERE_CIDATA_ISO_PATH="${VSPHERE_VM_NAME}/cidata.iso"
    $GOVC_CMD datastore.upload \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        "${_ci_iso_path}" \
        "${VSPHERE_CIDATA_ISO_PATH}"

    local _cdrom_device
    greenprint "🖥️ + 💿 Adding a CD-ROM device to the VM"
    _cdrom_device="$($GOVC_CMD device.cdrom.add \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        -vm "${VSPHERE_VM_NAME}")"

    greenprint "💿 Inserting the cloud-init ISO into the CD-ROM device"
    $GOVC_CMD device.cdrom.insert \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        -vm "${VSPHERE_VM_NAME}" \
        -device "${_cdrom_device}" \
        "${VSPHERE_CIDATA_ISO_PATH}"

    # start the VM
    greenprint "🔌 Powering up the VSphere VM"
    $GOVC_CMD vm.power \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        -on "${VSPHERE_VM_NAME}"

    HOST=$($GOVC_CMD vm.ip \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        "${VSPHERE_VM_NAME}")
    greenprint "⏱ Waiting for the VSphere VM to respond to ssh"
    _instanceWaitSSH "${HOST}"

    _ssh="ssh -oStrictHostKeyChecking=no -i ${_vsphere_ssh_key} $SSH_USER@$HOST"
    _instanceCheck "${_ssh}"

    greenprint "✅ Successfully verified VSphere image with cloud-init"
}
