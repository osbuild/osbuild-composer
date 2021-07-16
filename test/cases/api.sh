#!/usr/bin/bash

#
# Test osbuild-composer's main API endpoint by building a sample image and
# uploading it to the appropriate cloud provider. The test currently supports
# AWS and GCP.
#
# This script sets `-x` and is meant to always be run like that. This is
# simpler than adding extensive error reporting, which would make this script
# considerably more complex. Also, the full trace this produces is very useful
# for the primary audience: developers of osbuild-composer looking at the log
# from a run on a remote continuous integration system.
#

set -euxo pipefail

source /etc/os-release
DISTRO_CODE="${DISTRO_CODE:-${ID}_${VERSION_ID//./}}"

#TODO: remove this once there is rhel9 support for necessary image types
if [[ $DISTRO_CODE == rhel_90 ]]; then
    echo "Skipped"
    exit 0
fi

#
# Provision the software under tet.
#

/usr/libexec/osbuild-composer-test/provision.sh

#
# Which cloud provider are we testing?
#

CLOUD_PROVIDER_AWS="aws"
CLOUD_PROVIDER_GCP="gcp"
CLOUD_PROVIDER_AZURE="azure"
CLOUD_PROVIDER_AWS_S3="aws.s3"

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER_AWS}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    echo "Testing AWS"
    ;;
  "$CLOUD_PROVIDER_GCP")
    echo "Testing Google Cloud Platform"
    if [[ $ID == fedora ]]; then
        echo "Skipped, Fedora isn't supported by GCP"
        exit 0
    fi
    ;;
  "$CLOUD_PROVIDER_AZURE")
    echo "Testing Azure"
    ;;
  "$CLOUD_PROVIDER_AWS_S3")
    echo "Testing S3 bucket upload"
    if [[ $ID != "rhel" ]]; then
        echo "Skipped. S3 upload test is only tested on RHEL (testing only image type: rhel-edge-commit)."
        exit 0
    fi
    ;;
  *)
    echo "Unknown cloud provider '$CLOUD_PROVIDER'. Supported are '$CLOUD_PROVIDER_AWS', '$CLOUD_PROVIDER_AWS_S3', '$CLOUD_PROVIDER_GCP', '$CLOUD_PROVIDER_AZURE'"
    exit 1
    ;;
esac

#
# Verify that this script is running in the right environment.
#

# Check that needed variables are set to access AWS.
function checkEnvAWS() {
  printenv AWS_REGION AWS_BUCKET AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null
}

# Check that needed variables are set to access GCP.
function checkEnvGCP() {
  printenv GOOGLE_APPLICATION_CREDENTIALS GCP_BUCKET GCP_REGION GCP_API_TEST_SHARE_ACCOUNT > /dev/null
}

# Check that needed variables are set to access Azure.
function checkEnvAzure() {
  printenv AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID AZURE_RESOURCE_GROUP AZURE_LOCATION AZURE_CLIENT_ID AZURE_CLIENT_SECRET > /dev/null
}

# Check that needed variables are set to register to RHSM (RHEL only)
function checkEnvSubscription() {
  printenv API_TEST_SUBSCRIPTION_ORG_ID API_TEST_SUBSCRIPTION_ACTIVATION_KEY > /dev/null
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS" | "$CLOUD_PROVIDER_AWS_S3")
    checkEnvAWS
    ;;
  "$CLOUD_PROVIDER_GCP")
    checkEnvGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    checkEnvAzure
    ;;
esac
[[ "$ID" == "rhel" ]] && checkEnvSubscription

#
# Create a temporary directory and ensure it gets deleted when this script
# terminates in any way.
#

function cleanupAWS() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  AWS_CMD="${AWS_CMD:-}"
  AWS_INSTANCE_ID="${AWS_INSTANCE_ID:-}"
  AMI_IMAGE_ID="${AMI_IMAGE_ID:-}"
  AWS_SNAPSHOT_ID="${AWS_SNAPSHOT_ID:-}"

  if [ -n "$AWS_CMD" ]; then
    set +e
    $AWS_CMD ec2 terminate-instances --instance-ids "$AWS_INSTANCE_ID"
    $AWS_CMD ec2 deregister-image --image-id "$AMI_IMAGE_ID"
    $AWS_CMD ec2 delete-snapshot --snapshot-id "$AWS_SNAPSHOT_ID"
    $AWS_CMD ec2 delete-key-pair --key-name "key-for-$AMI_IMAGE_ID"
    set -e
  fi
}

function cleanupAWSS3() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # extract filename component from URL
  local S3_FILENAME
  S3_FILENAME=$(echo "${S3_URL}" | grep -oP '(?<=/)[^/]+(?=\?)')

  # prepend bucket
  local S3_URI
  S3_URI="s3://${AWS_BUCKET}/${S3_FILENAME}"

  # since this function can be called at any time, ensure that we don't expand unbound variables
  AWS_CMD="${AWS_CMD:-}"

  if [ -n "$AWS_CMD" ]; then
    set +e
    $AWS_CMD s3 rm "${S3_URI}"
    set -e
  fi
}

function cleanupGCP() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  GCP_CMD="${GCP_CMD:-}"
  GCP_IMAGE_NAME="${GCP_IMAGE_NAME:-}"
  GCP_INSTANCE_NAME="${GCP_INSTANCE_NAME:-}"
  GCP_ZONE="${GCP_ZONE:-}"

  if [ -n "$GCP_CMD" ]; then
    set +e
    $GCP_CMD compute instances delete --zone="$GCP_ZONE" "$GCP_INSTANCE_NAME"
    $GCP_CMD compute images delete "$GCP_IMAGE_NAME"
    set -e
  fi
}

function cleanupAzure() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  AZURE_CMD="${AZURE_CMD:-}"
  AZURE_IMAGE_NAME="${AZURE_IMAGE_NAME:-}"

  # do not run clean-up if the image name is not yet defined
  if [[ -n "$AZURE_CMD" && -n "$AZURE_IMAGE_NAME" ]]; then
    set +e
    # Re-get the vm_details in case the VM creation is failed.
    [ -f "$WORKDIR/vm_details.json" ] || "$AZURE_CMD" vm show --name "$AZURE_INSTANCE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" --show-details > "$WORKDIR/vm_details.json"
    # Get all the resources ids
    VM_ID=$(jq -r '.id' "$WORKDIR"/vm_details.json)
    OSDISK_ID=$(jq -r '.storageProfile.osDisk.managedDisk.id' "$WORKDIR"/vm_details.json)
    NIC_ID=$(jq -r '.networkProfile.networkInterfaces[0].id' "$WORKDIR"/vm_details.json)
    "$AZURE_CMD" network nic show --ids "$NIC_ID" > "$WORKDIR"/nic_details.json
    NSG_ID=$(jq -r '.networkSecurityGroup.id' "$WORKDIR"/nic_details.json)
    PUBLICIP_ID=$(jq -r '.ipConfigurations[0].publicIpAddress.id' "$WORKDIR"/nic_details.json)

    # Delete resources. Some resources must be removed in order:
    # - Delete VM prior to any other resources
    # - Delete NIC prior to NSG, public-ip
    # Left Virtual Network and Storage Account there because other tests in the same resource group will reuse them
    for id in "$VM_ID" "$OSDISK_ID" "$NIC_ID" "$NSG_ID" "$PUBLICIP_ID"; do
      echo "Deleting $id..."
      "$AZURE_CMD" resource delete --ids "$id"
    done

    # Delete image after VM deleting.
    $AZURE_CMD image delete --resource-group "$AZURE_RESOURCE_GROUP" --name "$AZURE_IMAGE_NAME"
    # find a storage account by its tag
    AZURE_STORAGE_ACCOUNT=$("$AZURE_CMD" resource list --tag imageBuilderStorageAccount=location="$AZURE_LOCATION" | jq -r .[0].name)
    AZURE_CONNECTION_STRING=$("$AZURE_CMD" storage account show-connection-string --name "$AZURE_STORAGE_ACCOUNT" | jq -r .connectionString)
    "$AZURE_CMD" storage blob delete --container-name imagebuilder --name "$AZURE_IMAGE_NAME".vhd --account-name "$AZURE_STORAGE_ACCOUNT" --connection-string "$AZURE_CONNECTION_STRING"
    set -e
  fi
}

WORKDIR=$(mktemp -d)
function cleanup() {
  case $CLOUD_PROVIDER in
    "$CLOUD_PROVIDER_AWS")
      cleanupAWS
      ;;
    "$CLOUD_PROVIDER_AWS_S3")
      cleanupAWSS3
      ;;
    "$CLOUD_PROVIDER_GCP")
      cleanupGCP
      ;;
    "$CLOUD_PROVIDER_AZURE")
      cleanupAzure
      ;;
  esac

  rm -rf "$WORKDIR"
}
trap cleanup EXIT

#
# Install the necessary cloud provider client tools
#

#
# Install the aws client from the upstream release, because it doesn't seem to
# be available as a RHEL package.
#
function installClientAWS() {
  if ! hash aws; then
    sudo dnf install -y awscli
    aws --version
  fi

  AWS_CMD="aws --region $AWS_REGION --output json --color on"
}

#
# Install the gcp clients from the upstream release
#
function installClientGCP() {
  if ! hash gcloud; then
    sudo tee -a /etc/yum.repos.d/google-cloud-sdk.repo << EOM
[google-cloud-sdk]
name=Google Cloud SDK
baseurl=https://packages.cloud.google.com/yum/repos/cloud-sdk-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg
       https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOM
  fi

  sudo dnf -y install google-cloud-sdk
  GCP_CMD="gcloud --format=json --quiet"
  $GCP_CMD --version
}

function installClientAzure() {
  if ! hash az; then
    # this installation method is taken from the official docs:
    # https://docs.microsoft.com/cs-cz/cli/azure/install-azure-cli-linux?pivots=dnf
    sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc
    echo -e "[azure-cli]
name=Azure CLI
baseurl=https://packages.microsoft.com/yumrepos/azure-cli
enabled=1
gpgcheck=1
gpgkey=https://packages.microsoft.com/keys/microsoft.asc" | sudo tee /etc/yum.repos.d/azure-cli.repo
  fi

  sudo dnf install -y azure-cli
  AZURE_CMD="az"
  $AZURE_CMD version
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS" | "$CLOUD_PROVIDER_AWS_S3")
    installClientAWS
    ;;
  "$CLOUD_PROVIDER_GCP")
    installClientGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    installClientAzure
    ;;
esac

#
# Make sure /openapi.json and /version endpoints return success
#

curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/composer/v1/version | jq .

curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/composer/v1/openapi.json | jq .

#
# Prepare a request to be sent to the composer API.
#

REQUEST_FILE="${WORKDIR}/request.json"
ARCH=$(uname -m)
SSH_USER=

# Generate a string, which can be used as a predictable resource name,
# especially when running the test in CI where we may need to clean up
# resources in case the test unexpectedly fails or is canceled
CI="${CI:-false}"
if [[ "$CI" == true ]]; then
  # in CI, imitate GenerateCIArtifactName() from internal/test/helpers.go
  TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_BUILD_ID"
else
  # if not running in Jenkins, generate ID not relying on specific env variables
  TEST_ID=$(uuidgen);
fi

case $(set +x; . /etc/os-release; echo "$ID-$VERSION_ID") in
  "rhel-8.4")
    DISTRO="rhel-84"
    SSH_USER="cloud-user"
    ;;
  "rhel-8.2" | "rhel-8.3")
    DISTRO="rhel-8"
    SSH_USER="cloud-user"
    ;;
  "fedora-33")
    DISTRO="fedora-33"
    SSH_USER="fedora"
    ;;
  "centos-8")
    DISTRO="centos-8"
    SSH_USER="cloud-user"
    ;;
esac

# Only RHEL need subscription block.
if [[ "$ID" == "rhel" ]]; then
  SUBSCRIPTION_BLOCK=$(cat <<EndOfMessage
,
    "subscription": {
      "organization": ${API_TEST_SUBSCRIPTION_ORG_ID:-},
      "activation-key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY:-}",
      "base-url": "https://cdn.redhat.com/",
      "server-url": "subscription.rhsm.redhat.com",
      "insights": true
    }
EndOfMessage
)
else
  SUBSCRIPTION_BLOCK=''
fi

function createReqFileAWS() {
  AWS_SNAPSHOT_NAME=$(uuidgen)

  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "packages": [
      "postgresql"
    ]${SUBSCRIPTION_BLOCK}
  },
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "ami",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "upload_request": {
          "type": "aws",
          "options": {
            "region": "${AWS_REGION}",
            "s3": {
              "access_key_id": "${AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${AWS_SECRET_ACCESS_KEY}",
              "bucket": "${AWS_BUCKET}"
            },
            "ec2": {
              "access_key_id": "${AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${AWS_SECRET_ACCESS_KEY}",
              "snapshot_name": "${AWS_SNAPSHOT_NAME}",
              "share_with_accounts": ["${AWS_API_TEST_SHARE_ACCOUNT}"]
            }
          }
      }
    }
  ]
}
EOF
}

function createReqFileAWSS3() {
  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "packages": [
      "postgresql"
    ]
  },
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "rhel-edge-commit",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "ostree": {
        "ref": "test/rhel/8/edge"
      },
      "upload_request": {
          "type": "aws.s3",
          "options": {
            "region": "${AWS_REGION}",
            "s3": {
              "access_key_id": "${AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${AWS_SECRET_ACCESS_KEY}",
              "bucket": "${AWS_BUCKET}"
            }
          }
      }
    }
  ]
}
EOF
}

function createReqFileGCP() {
  # constrains for GCP resource IDs:
  # - max 62 characters
  # - must be a match of regex '[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?|[1-9][0-9]{0,19}'
  #
  # use sha224sum to get predictable 56 characters long testID without invalid characters
  GCP_TEST_ID_HASH="$(echo -n "$TEST_ID" | sha224sum - | sed -E 's/([a-z0-9])\s+-/\1/')"

  GCP_IMAGE_NAME="image-$GCP_TEST_ID_HASH"

  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "packages": [
      "postgresql"
    ]${SUBSCRIPTION_BLOCK}
  },
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "vhd",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "upload_request": {
          "type": "gcp",
          "options": {
            "bucket": "${GCP_BUCKET}",
            "region": "${GCP_REGION}",
            "image_name": "${GCP_IMAGE_NAME}",
            "share_with_accounts": ["${GCP_API_TEST_SHARE_ACCOUNT}"]
          }
      }
    }
  ]
}
EOF
}

function createReqFileAzure() {
  AZURE_IMAGE_NAME="osbuild-composer-api-test-$(uuidgen)"

  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "packages": [
      "postgresql"
    ]${SUBSCRIPTION_BLOCK}
  },
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "vhd",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "upload_request": {
          "type": "azure",
          "options": {
            "tenant_id": "${AZURE_TENANT_ID}",
            "subscription_id": "${AZURE_SUBSCRIPTION_ID}",
            "resource_group": "${AZURE_RESOURCE_GROUP}",
            "location": "${AZURE_LOCATION}",
            "image_name": "${AZURE_IMAGE_NAME}"
          }
      }
    }
  ]
}
EOF
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    createReqFileAWS
    ;;
  "$CLOUD_PROVIDER_AWS_S3")
    createReqFileAWSS3
    ;;
  "$CLOUD_PROVIDER_GCP")
    createReqFileGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    createReqFileAzure
    ;;
esac

#
# Send the request and wait for the job to finish.
#
# Separate `curl` and `jq` commands here, because piping them together hides
# the server's response in case of an error.
#

function sendCompose() {
    OUTPUT=$(curl \
                 --silent \
                 --show-error \
                 --cacert /etc/osbuild-composer/ca-crt.pem \
                 --key /etc/osbuild-composer/client-key.pem \
                 --cert /etc/osbuild-composer/client-crt.pem \
                 --header 'Content-Type: application/json' \
                 --request POST \
                 --data @"$REQUEST_FILE" \
                 https://localhost/api/composer/v1/compose)

    COMPOSE_ID=$(echo "$OUTPUT" | jq -r '.id')
}

function waitForState() {
    local DESIRED_STATE="${1:-success}"
    while true
    do
        OUTPUT=$(curl \
                     --silent \
                     --show-error \
                     --cacert /etc/osbuild-composer/ca-crt.pem \
                     --key /etc/osbuild-composer/client-key.pem \
                     --cert /etc/osbuild-composer/client-crt.pem \
                     https://localhost/api/composer/v1/compose/"$COMPOSE_ID")

        COMPOSE_STATUS=$(echo "$OUTPUT" | jq -r '.image_status.status')
        UPLOAD_STATUS=$(echo "$OUTPUT" | jq -r '.image_status.upload_status.status')
        UPLOAD_TYPE=$(echo "$OUTPUT" | jq -r '.image_status.upload_status.type')
        UPLOAD_OPTIONS=$(echo "$OUTPUT" | jq -r '.image_status.upload_status.options')

        case "$COMPOSE_STATUS" in
            "$DESIRED_STATE")
                break
                ;;
            # all valid status values for a compose which hasn't finished yet
            "pending"|"building"|"uploading"|"registering")
                ;;
            # default undesired state
            "failure")
                echo "Image compose failed"
                exit 1
                ;;
            *)
                echo "API returned unexpected image_status.status value: '$COMPOSE_STATUS'"
                exit 1
                ;;
        esac

        sleep 30
    done
}

# a pending shouldn't state shouldn't trip up the heartbeats
sudo systemctl stop "osbuild-worker@*"
sendCompose
waitForState "pending"
# jobs time out after 2 minutes, so 180 seconds gives ample time to make sure it
# doesn't time out for pending jobs
sleep 180
waitForState "pending"

# crashed/stopped/killed worker should result in a failed state
sudo systemctl start "osbuild-worker@1"
waitForState "building"
sudo systemctl stop "osbuild-worker@*"
waitForState "failure"
sudo systemctl start "osbuild-worker@1"

# full integration case
sendCompose
waitForState
test "$UPLOAD_STATUS" = "success"
test "$UPLOAD_TYPE" = "$CLOUD_PROVIDER"


#
# Verify the Cloud-provider specific upload_status options
#

function checkUploadStatusOptionsAWS() {
  local AMI
  AMI=$(echo "$UPLOAD_OPTIONS" | jq -r '.ami')
  local REGION
  REGION=$(echo "$UPLOAD_OPTIONS" | jq -r '.region')

  # AWS ID consist of resource identifier followed by a 17-character string
  echo "$AMI" | grep -e 'ami-[[:alnum:]]\{17\}' -
  test "$REGION" = "$AWS_REGION"
}

function checkUploadStatusOptionsAWSS3() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # S3 URL contains region and bucket name
  echo "$S3_URL" | grep -F "$AWS_BUCKET" -
  echo "$S3_URL" | grep -F "$AWS_REGION" -
}

function checkUploadStatusOptionsGCP() {
  GCP_PROJECT=$(jq -r '.project_id' "$GOOGLE_APPLICATION_CREDENTIALS")

  local IMAGE_NAME
  IMAGE_NAME=$(echo "$UPLOAD_OPTIONS" | jq -r '.image_name')
  local PROJECT_ID
  PROJECT_ID=$(echo "$UPLOAD_OPTIONS" | jq -r '.project_id')

  test "$IMAGE_NAME" = "$GCP_IMAGE_NAME"
  test "$PROJECT_ID" = "$GCP_PROJECT"
}

function checkUploadStatusOptionsAzure() {
  local IMAGE_NAME
  IMAGE_NAME=$(echo "$UPLOAD_OPTIONS" | jq -r '.image_name')

  test "$IMAGE_NAME" = "$AZURE_IMAGE_NAME"
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    checkUploadStatusOptionsAWS
    ;;
  "$CLOUD_PROVIDER_AWS_S3")
    checkUploadStatusOptionsAWSS3
    ;;
  "$CLOUD_PROVIDER_GCP")
    checkUploadStatusOptionsGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    checkUploadStatusOptionsAzure
    ;;
esac

#
# Verify the image landed in the appropriate cloud provider, and delete it.
#

# Reusable function, which waits for a given host to respond to SSH
function _instanceWaitSSH() {
  local HOST="$1"

  for LOOP_COUNTER in {0..30}; do
      if ssh-keyscan "$HOST" > /dev/null 2>&1; then
          echo "SSH is up!"
          # ssh-keyscan "$PUBLIC_IP" | sudo tee -a /root/.ssh/known_hosts
          break
      fi
      echo "Retrying in 5 seconds... $LOOP_COUNTER"
      sleep 5
  done
}

function _instanceCheck() {
  echo "✔️ Instance checking"
  local _ssh="$1"

  # Check if postgres is installed
  $_ssh rpm -q postgresql

  # Verify subscribe status. Loop check since the system may not be registered such early(RHEL only)
  if [[ "$ID" == "rhel" ]]; then
    set +eu
    for LOOP_COUNTER in {1..10}; do
        subscribe_org_id=$($_ssh sudo subscription-manager identity | grep 'org ID')
        if [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]; then
            echo "System is subscribed."
            break
        else
            echo "System is not subscribed. Retrying in 30 seconds...($LOOP_COUNTER/10)"
            sleep 30
        fi
    done
    set -eu
    [[ "$subscribe_org_id" == "org ID: $API_TEST_SUBSCRIPTION_ORG_ID" ]]

    # Unregister subscription
    $_ssh sudo subscription-manager unregister
  else
    echo "Not RHEL OS. Skip subscription check."
  fi
}

# Verify image in EC2 on AWS
function verifyInAWS() {
  $AWS_CMD ec2 describe-images \
    --owners self \
    --filters Name=name,Values="$AWS_SNAPSHOT_NAME" \
    > "$WORKDIR/ami.json"

  AMI_IMAGE_ID=$(jq -r '.Images[].ImageId' "$WORKDIR/ami.json")
  AWS_SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' "$WORKDIR/ami.json")
  SHARE_OK=1

  # Verify that the ec2 snapshot was shared
  $AWS_CMD ec2 describe-snapshot-attribute --snapshot-id "$AWS_SNAPSHOT_ID" --attribute createVolumePermission > "$WORKDIR/snapshot-attributes.json"

  SHARED_ID=$(jq -r '.CreateVolumePermissions[0].UserId' "$WORKDIR/snapshot-attributes.json")
  if [ "$AWS_API_TEST_SHARE_ACCOUNT" != "$SHARED_ID" ]; then
    SHARE_OK=0
  fi

  # Verify that the ec2 ami was shared
  $AWS_CMD ec2 describe-image-attribute --image-id "$AMI_IMAGE_ID" --attribute launchPermission > "$WORKDIR/ami-attributes.json"

  SHARED_ID=$(jq -r '.LaunchPermissions[0].UserId' "$WORKDIR/ami-attributes.json")
  if [ "$AWS_API_TEST_SHARE_ACCOUNT" != "$SHARED_ID" ]; then
    SHARE_OK=0
  fi

  if [ "$SHARE_OK" != 1 ]; then
    echo "EC2 snapshot wasn't shared with the AWS_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
  fi

  # Create key-pair
  $AWS_CMD ec2 create-key-pair --key-name "key-for-$AMI_IMAGE_ID" --query 'KeyMaterial' --output text > keypair.pem
  chmod 400 ./keypair.pem

  # Create an instance based on the ami
  $AWS_CMD ec2 run-instances --image-id "$AMI_IMAGE_ID" --count 1 --instance-type t2.micro --key-name "key-for-$AMI_IMAGE_ID" > "$WORKDIR/instances.json"
  AWS_INSTANCE_ID=$(jq -r '.Instances[].InstanceId' "$WORKDIR/instances.json")

  $AWS_CMD ec2 wait instance-running --instance-ids "$AWS_INSTANCE_ID"

  $AWS_CMD ec2 describe-instances --instance-ids "$AWS_INSTANCE_ID" > "$WORKDIR/instances.json"
  HOST=$(jq -r '.Reservations[].Instances[].PublicIpAddress' "$WORKDIR/instances.json")

  echo "⏱ Waiting for AWS instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i ./keypair.pem $SSH_USER@$HOST"
  _instanceCheck "$_ssh"
}


# Verify tarball in AWS S3
function verifyInAWSS3() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # Download the commit using the Presigned URL
  curl "${S3_URL}" --output "${WORKDIR}/edge-commit.tar"

  # Verify that the commit contains the ref we defined in the request
  tar tvf "${WORKDIR}/edge-commit.tar" "repo/refs/heads/test/rhel/8/edge"

  # verify that the commit hash matches the metadata
  local API_COMMIT_ID
  API_COMMIT_ID=$(curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/composer/v1/compose/"$COMPOSE_ID"/metadata | jq -r '.ostree_commit')

  local TAR_COMMIT_ID
  TAR_COMMIT_ID=$(tar xf "${WORKDIR}/edge-commit.tar" "repo/refs/heads/test/rhel/8/edge" -O)

  if [[ "${API_COMMIT_ID}" != "${TAR_COMMIT_ID}" ]]; then
      echo "Commit ID returned from API does not match Commit ID in archive 😠"
      exit 1
  fi
}

# Verify image in Compute Engine on GCP
function verifyInGCP() {
  # Authenticate
  $GCP_CMD auth activate-service-account --key-file "$GOOGLE_APPLICATION_CREDENTIALS"
  # Extract and set the default project to be used for commands
  GCP_PROJECT=$(jq -r '.project_id' "$GOOGLE_APPLICATION_CREDENTIALS")
  $GCP_CMD config set project "$GCP_PROJECT"

  # Verify that the image was shared
  SHARE_OK=1
  $GCP_CMD compute images get-iam-policy "$GCP_IMAGE_NAME" > "$WORKDIR/image-iam-policy.json"
  SHARED_ACCOUNT=$(jq -r '.bindings[0].members[0]' "$WORKDIR/image-iam-policy.json")
  SHARED_ROLE=$(jq -r '.bindings[0].role' "$WORKDIR/image-iam-policy.json")
  if [ "$SHARED_ACCOUNT" != "$GCP_API_TEST_SHARE_ACCOUNT" ] || [ "$SHARED_ROLE" != "roles/compute.imageUser" ]; then
    SHARE_OK=0
  fi

  if [ "$SHARE_OK" != 1 ]; then
    echo "GCP image wasn't shared with the GCP_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
  fi

  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  GCP_SSH_KEY="$WORKDIR/id_google_compute_engine"
  ssh-keygen -t rsa -f "$GCP_SSH_KEY" -C "$SSH_USER" -N ""
  GCP_SSH_METADATA_FILE="$WORKDIR/gcp-ssh-keys-metadata"

  echo "${SSH_USER}:$(cat "$GCP_SSH_KEY".pub)" > "$GCP_SSH_METADATA_FILE"

  # create the instance
  # resource ID can have max 62 characters, the $GCP_TEST_ID_HASH contains 56 characters
  GCP_INSTANCE_NAME="vm-$GCP_TEST_ID_HASH"

  # Randomize the used GCP zone to prevent hitting "exhausted resources" error on each test re-run
  # disable Shellcheck error as the suggested alternatives are less readable for this use case
  # shellcheck disable=SC2207
  local GCP_ZONES=($($GCP_CMD compute zones list --filter="region=$GCP_REGION" | jq '.[] | select(.status == "UP") | .name' | tr -d '"' | tr '\n' ' '))
  GCP_ZONE=${GCP_ZONES[$((RANDOM % ${#GCP_ZONES[@]}))]}

  $GCP_CMD compute instances create "$GCP_INSTANCE_NAME" \
    --zone="$GCP_ZONE" \
    --image-project="$GCP_PROJECT" \
    --image="$GCP_IMAGE_NAME" \
    --metadata-from-file=ssh-keys="$GCP_SSH_METADATA_FILE"
  HOST=$($GCP_CMD compute instances describe "$GCP_INSTANCE_NAME" --zone="$GCP_ZONE" --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

  echo "⏱ Waiting for GCP instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i $GCP_SSH_KEY $SSH_USER@$HOST"
  _instanceCheck "$_ssh"
}

# Verify image in Azure
function verifyInAzure() {
  set +x
  $AZURE_CMD login --service-principal --username "${AZURE_CLIENT_ID}" --password "${AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"
  set -x

  # verify that the image exists
  $AZURE_CMD image show --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}"

  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  AZURE_SSH_KEY="$WORKDIR/id_azure"
  ssh-keygen -t rsa -f "$AZURE_SSH_KEY" -C "$SSH_USER" -N ""

  # create the instance
  AZURE_INSTANCE_NAME="vm-$(uuidgen)"
  $AZURE_CMD vm create --name "$AZURE_INSTANCE_NAME" \
    --resource-group "$AZURE_RESOURCE_GROUP" \
    --image "$AZURE_IMAGE_NAME" \
    --size "Standard_B1s" \
    --admin-username "$SSH_USER" \
    --ssh-key-values "$AZURE_SSH_KEY.pub" \
    --authentication-type "ssh" \
    --location "$AZURE_LOCATION"
  $AZURE_CMD vm show --name "$AZURE_INSTANCE_NAME" --resource-group "$AZURE_RESOURCE_GROUP" --show-details > "$WORKDIR/vm_details.json"
  HOST=$(jq -r '.publicIps' "$WORKDIR/vm_details.json")

  echo "⏱  Waiting for Azure instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i $AZURE_SSH_KEY $SSH_USER@$HOST"
  _instanceCheck "$_ssh"
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    verifyInAWS
    ;;
  "$CLOUD_PROVIDER_AWS_S3")
    verifyInAWSS3
    ;;
  "$CLOUD_PROVIDER_GCP")
    verifyInGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    verifyInAzure
    ;;
esac

# Verify selected package (postgresql) is included in package list
function verifyPackageList() {
  local PACKAGENAMES
  PACKAGENAMES=$(curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/composer/v1/compose/"$COMPOSE_ID"/metadata | jq -r '.packages[].name')

  if ! grep -q postgresql <<< "${PACKAGENAMES}"; then
      echo "'postgresql' not found in compose package list 😠"
      exit 1
  fi
}

verifyPackageList

# Verify the identityfilter
cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
[koji]
allowed_domains = [ "localhost", "client.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"

[worker]
allowed_domains = [ "localhost", "worker.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"

[composer_api]
identity_filter = ["000000"]
EOF

sudo systemctl restart osbuild-composer

# account number 000000
VALIDAUTHSTRING="eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDAiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAwIn19fQ=="
# account number 000001
INVALIDAUTHSTRING="eyJlbnRpdGxlbWVudHMiOnsiaW5zaWdodHMiOnsiaXNfZW50aXRsZWQiOnRydWV9LCJzbWFydF9tYW5hZ2VtZW50Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwib3BlbnNoaWZ0Ijp7ImlzX2VudGl0bGVkIjp0cnVlfSwiaHlicmlkIjp7ImlzX2VudGl0bGVkIjp0cnVlfSwibWlncmF0aW9ucyI6eyJpc19lbnRpdGxlZCI6dHJ1ZX0sImFuc2libGUiOnsiaXNfZW50aXRsZWQiOnRydWV9fSwiaWRlbnRpdHkiOnsiYWNjb3VudF9udW1iZXIiOiIwMDAwMDMiLCJ0eXBlIjoiVXNlciIsInVzZXIiOnsidXNlcm5hbWUiOiJ1c2VyIiwiZW1haWwiOiJ1c2VyQHVzZXIudXNlciIsImZpcnN0X25hbWUiOiJ1c2VyIiwibGFzdF9uYW1lIjoidXNlciIsImlzX2FjdGl2ZSI6dHJ1ZSwiaXNfb3JnX2FkbWluIjp0cnVlLCJpc19pbnRlcm5hbCI6dHJ1ZSwibG9jYWxlIjoiZW4tVVMifSwiaW50ZXJuYWwiOnsib3JnX2lkIjoiMDAwMDAwIn19fQo="

curl \
    --silent \
    --show-error \
    --header "x-rh-identity: $VALIDAUTHSTRING" \
    http://localhost:443/api/composer/v1/version | jq .

#
# Make sure the invalid auth string returns a 404
#
[ "$(curl \
        --silent \
        --output /dev/null \
        --write-out '%{http_code}' \
        --header "x-rh-identity: $INVALIDAUTHSTRING" \
        http://localhost:443/api/composer/v1/version)" = "404" ]

#
# Make sure that requesting a non existing paquet returns a 400 error
#
[ "$(curl \
    --silent \
    --output /dev/null \
    --write-out '%{http_code}' \
    --header "x-rh-identity: $VALIDAUTHSTRING" \
    -H "Content-Type: application/json" \
    -d '{ "distribution": "centos-8", "image_requests": [ { "architecture": "x86_64", "image_type": "ami", "repositories": [ { "baseurl": "http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/", "rhsm": false }, { "baseurl": "http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/", "rhsm": false }, { "baseurl": "http://mirror.centos.org/centos/8-stream/extras/x86_64/os/", "rhsm": false } ], "upload_request": { "type": "aws.s3", "options": { "region": "somewhere", "s3": { "access_key_id": "thingy", "secret_access_key": "thing", "bucket": "thingything" } } } } ], "customizations": { "packages": [ "jesuisunpaquetquinexistepas_idonotexist" ] } }' \
    http://localhost:443/api/composer/v1/compose)" = "400" ]

#
# Make sure that a request that makes the dnf-json crash returns a 500 error
#
sudo cp -f /usr/libexec/osbuild-composer/dnf-json /usr/libexec/osbuild-composer/dnf-json.bak
sudo cat << EOF | sudo tee /usr/libexec/osbuild-composer/dnf-json
#!/usr/bin/python3
raise Exception()
EOF
[ "$(curl \
    --silent \
    --output /dev/null \
    --write-out '%{http_code}' \
    --header "x-rh-identity: $VALIDAUTHSTRING" \
    -H "Content-Type: application/json" \
    -d '{ "distribution": "centos-8", "image_requests": [ { "architecture": "x86_64", "image_type": "ami", "repositories": [ { "baseurl": "http://mirror.centos.org/centos/8-stream/BaseOS/x86_64/os/", "rhsm": false }, { "baseurl": "http://mirror.centos.org/centos/8-stream/AppStream/x86_64/os/", "rhsm": false }, { "baseurl": "http://mirror.centos.org/centos/8-stream/extras/x86_64/os/", "rhsm": false } ], "upload_request": { "type": "aws.s3", "options": { "region": "somewhere", "s3": { "access_key_id": "thingy", "secret_access_key": "thing", "bucket": "thingything" } } } } ], "customizations": { "packages": [ "jesuisunpaquetquinexistepas_idonotexist" ] } }' \
    http://localhost:443/api/composer/v1/compose)" = "500" ]

sudo mv -f /usr/libexec/osbuild-composer/dnf-json.bak /usr/libexec/osbuild-composer/dnf-json

exit 0
