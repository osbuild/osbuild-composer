#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/common.sh

# Check that needed variables are set to access Azure.
function checkEnv() {
  printenv AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID AZURE_RESOURCE_GROUP AZURE_LOCATION V2_AZURE_CLIENT_ID V2_AZURE_CLIENT_SECRET > /dev/null
}

function cleanup() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  AZURE_CMD="${AZURE_CMD:-}"
  AZURE_IMAGE_NAME="${AZURE_IMAGE_NAME:-}"
  AZURE_INSTANCE_NAME="${AZURE_INSTANCE_NAME:-}"

  # do not run clean-up if the image name is not yet defined
  if [[ -n "$AZURE_CMD" && -n "$AZURE_IMAGE_NAME" ]]; then
    # Re-get the vm_details in case the VM creation is failed.
    [ -f "$WORKDIR/vm_details.json" ] || $AZURE_CMD vm show --name "$AZURE_INSTANCE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" --show-details > "$WORKDIR/vm_details.json"
    # Get all the resources ids
    VM_ID=$(jq -r '.id' "$WORKDIR"/vm_details.json)
    OSDISK_ID=$(jq -r '.storageProfile.osDisk.managedDisk.id' "$WORKDIR"/vm_details.json)
    NIC_ID=$(jq -r '.networkProfile.networkInterfaces[0].id' "$WORKDIR"/vm_details.json)
    $AZURE_CMD network nic show --ids "$NIC_ID" > "$WORKDIR"/nic_details.json
    NSG_ID=$(jq -r '.networkSecurityGroup.id' "$WORKDIR"/nic_details.json)
    PUBLICIP_ID=$(jq -r '.ipConfigurations[0].publicIpAddress.id' "$WORKDIR"/nic_details.json)

    # Delete resources. Some resources must be removed in order:
    # - Delete VM prior to any other resources
    # - Delete NIC prior to NSG, public-ip
    # Left Virtual Network and Storage Account there because other tests in the same resource group will reuse them
    for id in "$VM_ID" "$OSDISK_ID" "$NIC_ID" "$NSG_ID" "$PUBLICIP_ID"; do
      echo "Deleting $id..."
      $AZURE_CMD resource delete --ids "$id"
    done

    # Delete image after VM deleting.
    $AZURE_CMD image delete --resource-group "$AZURE_RESOURCE_GROUP" --name "$AZURE_IMAGE_NAME"
    # find a storage account by its tag
    AZURE_STORAGE_ACCOUNT=$($AZURE_CMD resource list --tag imageBuilderStorageAccount=location="$AZURE_LOCATION" | jq -r .[0].name)
    AZURE_CONNECTION_STRING=$($AZURE_CMD storage account show-connection-string --name "$AZURE_STORAGE_ACCOUNT" | jq -r .connectionString)
    $AZURE_CMD storage blob delete --container-name imagebuilder --name "$AZURE_IMAGE_NAME".vhd --account-name "$AZURE_STORAGE_ACCOUNT" --connection-string "$AZURE_CONNECTION_STRING"
  fi
}

function installClient() {
  if ! hash az; then
    echo "Using 'azure-cli' from a container"
    sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_IMAGE_CLOUD_TOOLS}"

    # directory mounted to the container, in which azure-cli stores the credentials after logging in
    AZURE_CMD_CREDS_DIR="${WORKDIR}/azure-cli_credentials"
    mkdir "${AZURE_CMD_CREDS_DIR}"

    AZURE_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -v ${AZURE_CMD_CREDS_DIR}:/root/.azure:Z \
      -v ${WORKDIR}:${WORKDIR}:Z \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} az"
  else
    echo "Using pre-installed 'azure-cli' from the system"
    AZURE_CMD="az"
  fi
  $AZURE_CMD version
}

function createReqFile() {
  AZURE_IMAGE_NAME="image-$TEST_ID"

  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "filesystem": [
      {
        "mountpoint": "/var",
        "min_size": 262144000
      }
    ],
    "payload_repositories": [
      {
        "baseurl": "$PAYLOAD_REPO_URL"
      }
    ],
    "packages": [
      "postgresql",
      "dummy"
    ]${SUBSCRIPTION_BLOCK}${DIR_FILES_CUSTOMIZATION_BLOCK}${REPOSITORY_CUSTOMIZATION_BLOCK}${OPENSCAP_CUSTOMIZATION_BLOCK}
${TIMEZONE_CUSTOMIZATION_BLOCK}${FIREWALL_CUSTOMIZATION_BLOCK}${RPM_CUSTOMIZATION_BLOCK}${RHSM_CUSTOMIZATION_BLOCK}
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "upload_options": {
      "tenant_id": "${AZURE_TENANT_ID}",
      "subscription_id": "${AZURE_SUBSCRIPTION_ID}",
      "resource_group": "${AZURE_RESOURCE_GROUP}",
      "image_name": "${AZURE_IMAGE_NAME}",
      "hyper_v_generation": "V2"
    }
  }
}
EOF
}

function checkUploadStatusOptions() {
  local IMAGE_NAME
  IMAGE_NAME=$(echo "$UPLOAD_OPTIONS" | jq -r '.image_name')

  test "$IMAGE_NAME" = "$AZURE_IMAGE_NAME"
}

# Log into Azure
function cloud_login() {
  $AZURE_CMD login --service-principal --username "${V2_AZURE_CLIENT_ID}" --password "${V2_AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"
}

# Verify image in Azure
function verify() {
  cloud_login

  # verify that the image exists and tag it
  $AZURE_CMD image show --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}"
  $AZURE_CMD image update --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}" --tags gitlab-ci-test=true

  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  AZURE_SSH_KEY="$WORKDIR/id_azure"
  ssh-keygen -t rsa-sha2-512 -f "$AZURE_SSH_KEY" -C "$SSH_USER" -N ""

  # Create network resources with predictable names
  $AZURE_CMD network nsg create --resource-group "$AZURE_RESOURCE_GROUP" --name "nsg-$TEST_ID" --location "$AZURE_LOCATION" --tags gitlab-ci-test=true
  $AZURE_CMD network nsg rule create --resource-group "$AZURE_RESOURCE_GROUP" \
      --nsg-name "nsg-$TEST_ID" \
      --name SSH \
      --priority 1001 \
      --access Allow \
      --protocol Tcp \
      --destination-address-prefixes '*' \
      --destination-port-ranges 22 \
      --source-port-ranges '*' \
      --source-address-prefixes '*'
  $AZURE_CMD network vnet create --resource-group "$AZURE_RESOURCE_GROUP" \
      --name "vnet-$TEST_ID" \
      --subnet-name "snet-$TEST_ID" \
      --location "$AZURE_LOCATION" \
      --tags gitlab-ci-test=true
  $AZURE_CMD network public-ip create --resource-group "$AZURE_RESOURCE_GROUP" --name "ip-$TEST_ID" --location "$AZURE_LOCATION" --tags gitlab-ci-test=true
  $AZURE_CMD network nic create --resource-group "$AZURE_RESOURCE_GROUP" \
      --name "iface-$TEST_ID" \
      --subnet "snet-$TEST_ID" \
      --vnet-name "vnet-$TEST_ID" \
      --network-security-group "nsg-$TEST_ID" \
      --public-ip-address "ip-$TEST_ID" \
      --location "$AZURE_LOCATION" \
      --tags gitlab-ci-test=true

  # create the instance
  AZURE_INSTANCE_NAME="vm-$TEST_ID"
  $AZURE_CMD vm create --name "$AZURE_INSTANCE_NAME" \
    --resource-group "$AZURE_RESOURCE_GROUP" \
    --image "$AZURE_IMAGE_NAME" \
    --size "Standard_B1s" \
    --admin-username "$SSH_USER" \
    --ssh-key-values "$AZURE_SSH_KEY.pub" \
    --authentication-type "ssh" \
    --location "$AZURE_LOCATION" \
    --nics "iface-$TEST_ID" \
    --os-disk-name "disk-$TEST_ID" \
    --tags gitlab-ci-test=true
  $AZURE_CMD vm show --name "$AZURE_INSTANCE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" --show-details > "$WORKDIR/vm_details.json"
  HOST=$(jq -r '.publicIps' "$WORKDIR/vm_details.json")

  echo "⏱  Waiting for Azure instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i $AZURE_SSH_KEY $SSH_USER@$HOST"
  _instanceCheck "$_ssh"
}
