#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/aws.sh
source /usr/libexec/tests/osbuild-composer/api/common/common.sh

function checkEnv() {
    printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null
}

function cleanup() {
    greenprint "🧼 Cleaning up OCI"
    $OCI_CMD compute instance terminate --instance-id "${INSTANCE_ID}" --force
    $OCI_CMD compute image delete --image-id "${OCI_IMAGE_ID}" --force
}

# Set up temporary files.
TEMPDIR=$(mktemp -d)
OCI_CONFIG=${TEMPDIR}/oci-config
SSH_DATA_DIR=$(tools/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa

OCI_USER=$(jq -r '.user' "$OCI_SECRETS")
OCI_TENANCY=$(jq -r '.tenancy' "$OCI_SECRETS")
OCI_REGION=$(jq -r '.region' "$OCI_SECRETS")
OCI_FINGERPRINT=$(jq -r '.fingerprint' "$OCI_SECRETS")
OCI_COMPARTMENT=$(jq -r '.compartment' "$OCI_SECRETS")
OCI_SUBNET=$(jq -r '.subnet' "$OCI_SECRETS")

# copy private key to what oci considers a valid path
cp -p "$OCI_PRIVATE_KEY" "$TEMPDIR/priv_key.pem"
tee "$OCI_CONFIG" > /dev/null << EOF
[DEFAULT]
user=${OCI_USER}
fingerprint=${OCI_FINGERPRINT}
key_file=${TEMPDIR}/priv_key.pem
tenancy=${OCI_TENANCY}
region=${OCI_REGION}
EOF

function installClient() {
    if ! hash oci; then
        echo "Using 'oci' from a container"
        sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_IMAGE_CLOUD_TOOLS}"

        # OCI_CLI_AUTH
        OCI_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        -v ${SSH_DATA_DIR}:${SSH_DATA_DIR}:Z \
        -v ${OCI_PRIVATE_KEY}:${OCI_PRIVATE_KEY}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} /root/bin/oci --config-file $OCI_CONFIG --region $OCI_REGION --output json"
    else
        echo "Using pre-installed 'oci' from the system"
        OCI_CMD="oci --config-file $OCI_CONFIG --region $OCI_REGION"
    fi
    $OCI_CMD --version
    $OCI_CMD setup repair-file-permissions --file "${TEMPDIR}/priv_key.pem"
    $OCI_CMD setup repair-file-permissions --file "$OCI_CONFIG"
}

function createReqFile() {
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
      "nodejs",
      "postgresql",
      "dummy"
    ]${SUBSCRIPTION_BLOCK}${DIR_FILES_CUSTOMIZATION_BLOCK}${REPOSITORY_CUSTOMIZATION_BLOCK}${OPENSCAP_CUSTOMIZATION_BLOCK}
${TIMEZONE_CUSTOMIZATION_BLOCK}${CACERTS_CUSTOMIZATION_BLOCK}${ENABLED_MODULES_BLOCK}
  },
  "image_request": {
      "architecture": "$ARCH",
      "image_type": "${IMAGE_TYPE}",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "upload_options": {}
  }
}
EOF
}


function checkUploadStatusOptions() {
    local URL
    URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
    echo "$URL" | grep -qF "$OCI_REGION" -
}


function get_availability_domain_by_shape {
    for ad in $($OCI_CMD iam availability-domain list -c "$OCI_COMPARTMENT" | jq -r '.data[].name');do
        if [ "$($OCI_CMD compute shape list -c "$OCI_COMPARTMENT" --availability-domain "$ad" | jq --arg SHAPE "$1" -r '.data[]|select(.shape==$SHAPE)|.shape')" == "$1" ];then
            echo "$ad"
            return
        fi
    done
    return 1
}

# Verify image in OCI
function verify() {
    # import image
    echo "verifying oci image"
    URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
    OCI_IMAGE_DATA=$($OCI_CMD compute image import from-object-uri \
                        -c "$OCI_COMPARTMENT" \
                        --uri "$URL")
    echo "oci image data: $OCI_IMAGE_DATA"
    OCI_IMAGE_ID=$(echo "$OCI_IMAGE_DATA" | jq -r '.data.id')

    for LOOP_COUNTER in {0..120}; do
        STATE=$($OCI_CMD compute image get --image-id "$OCI_IMAGE_ID" | jq -r '.data["lifecycle-state"]')
        if [ "$STATE" = "AVAILABLE" ]; then
            echo "👻 the VM imported in time!"
            break
        fi
        if [ "$LOOP_COUNTER" = "120" ]; then
            echo "😞 the VM did not import in time ;_;"
            exit 1
        fi
        sleep 15
    done

    echo "adding compatibility schema to image"
    tee "$TEMPDIR/compat-schema.json" > /dev/null <<EOF
{
  "Storage.BootVolumeType": {
    "defaultValue": "PARAVIRTUALIZED",
    "descriptorType": "enumstring",
    "source": "IMAGE",
    "values": [
      "PARAVIRTUALIZED"
    ]
  }
}
EOF
    GLOBAL_SCHEMA=$($OCI_CMD compute global-image-capability-schema list | jq -r '.data[0]["current-version-name"]')
    $OCI_CMD compute image-capability-schema create \
            --compartment-id "$OCI_COMPARTMENT" \
            --image-id "$OCI_IMAGE_ID" \
            --global-image-capability-schema-version-name "$GLOBAL_SCHEMA" \
            --schema-data "file://$TEMPDIR/compat-schema.json"

    SHAPE="VM.Standard.E4.Flex"
    OCI_AVAILABILITY_DOMAIN="$(get_availability_domain_by_shape "$SHAPE")"
    INSTANCE=$($OCI_CMD compute instance launch  \
                        --shape "$SHAPE" \
                        --shape-config '{"memoryInGBs": 2.0, "ocpus": 1.0}' \
                        -c "${OCI_COMPARTMENT}" \
                        --availability-domain "${OCI_AVAILABILITY_DOMAIN}" \
                        --subnet-id "${OCI_SUBNET}" \
                        --image-id "${OCI_IMAGE_ID}" \
                        --freeform-tags "{\"Name\": \"${TEST_ID}\", \"gitlab-ci-test\": \"true\"}" \
                        --user-data-file "${SSH_DATA_DIR}/user-data")
    echo "Attempted to launch instance: $INSTANCE"
    INSTANCE_ID="$(echo "$INSTANCE" | jq -r '.data.id')"

    while true; do
        INSTANCE=$($OCI_CMD compute instance get --instance-id "$INSTANCE_ID")
        if [[ $(echo "$INSTANCE" | jq -r '.data["lifecycle-state"]') == RUNNING ]]; then
            break
        fi
        sleep 10
    done

    # Get data about the instance we built.
    PUBLIC_IP=$($OCI_CMD compute instance list-vnics --instance-id "$INSTANCE_ID" |  jq -r '.data[0]["public-ip"]')



    echo "⏱  Waiting for instance to respond to ssh"
    _instanceWaitSSH "$PUBLIC_IP"

    # Verify image
    _ssh="ssh -oStrictHostKeyChecking=no -i $SSH_KEY redhat@$PUBLIC_IP"
    _instanceCheck "$_ssh"
}
