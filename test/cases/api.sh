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

CLOUD_PROVIDER=${1:-$CLOUD_PROVIDER_AWS}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    echo "Testing AWS"
    ;;
  "$CLOUD_PROVIDER_GCP")
    echo "Testing Google Cloud Platform"
    ;;
  "$CLOUD_PROVIDER_AZURE")
    echo "Testing Azure"
    ;;
  *)
    echo "Unknown cloud provider '$CLOUD_PROVIDER'. Supported are '$CLOUD_PROVIDER_AWS', '$CLOUD_PROVIDER_GCP'"
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

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    checkEnvAWS
    ;;
  "$CLOUD_PROVIDER_GCP")
    checkEnvGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    checkEnvAzure
    ;;
esac

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

function cleanupGCP() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  GCP_CMD="${GCP_CMD:-}"
  GCP_IMAGE_NAME="${GCP_IMAGE_NAME:-}"
  GCP_INSTANCE_NAME="${GCP_INSTANCE_NAME:-}"

  if [ -n "$GCP_CMD" ]; then
    set +e
    $GCP_CMD compute instances delete --zone="$GCP_REGION-a" "$GCP_INSTANCE_NAME"
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
    $AZURE_CMD image delete --resource-group sharing-research --name "$AZURE_IMAGE_NAME"

    # find a storage account by its tag
    AZURE_STORAGE_ACCOUNT=$($AZURE_CMD resource list --tag imageBuilderStorageAccount=location="$AZURE_LOCATION" | jq -r .[0].name)
    $AZURE_CMD storage blob delete --container-name imagebuilder --name "$AZURE_IMAGE_NAME".vhd --account-name "$AZURE_STORAGE_ACCOUNT"
    set -e
  fi
}

WORKDIR=$(mktemp -d)
function cleanup() {
  case $CLOUD_PROVIDER in
    "$CLOUD_PROVIDER_AWS")
      cleanupAWS
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
    mkdir "$WORKDIR/aws"
    pushd "$WORKDIR/aws"
      curl -Ls --retry 5 --output awscliv2.zip \
        https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip
      unzip awscliv2.zip > /dev/null
      sudo ./aws/install > /dev/null
      aws --version
    popd
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
  "$CLOUD_PROVIDER_AWS")
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
# especially when running the test in Jenkins where we may need to clean up
# resources in case the test unexpectedly fails or is canceled
JENKINS_HOME="${JENKINS_HOME:-}"
if [[ -n "$JENKINS_HOME" ]]; then
  # in Jenkins, imitate GenerateCIArtifactName() from internal/test/helpers.go
  TEST_ID="$DISTRO_CODE-$ARCH-$BRANCH_NAME-$BUILD_ID"
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
  "fedora-32")
    DISTRO="fedora-32"
    SSH_USER="fedora"
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

function createReqFileAWS() {
  AWS_SNAPSHOT_NAME=$(uuidgen)

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
    ]
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

  if [[ "$COMPOSE_STATUS" != "pending" && "$COMPOSE_STATUS" != "running" ]]; then
    test "$COMPOSE_STATUS" = "success"
    test "$UPLOAD_STATUS" = "success"
    test "$UPLOAD_TYPE" = "$CLOUD_PROVIDER"
    break
  fi

  sleep 30
done

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

  # Check if postgres is installed
  ssh -oStrictHostKeyChecking=no -i ./keypair.pem "$SSH_USER"@"$HOST" rpm -q postgresql

  if [ "$SHARE_OK" != 1 ]; then
    echo "EC2 snapshot wasn't shared with the AWS_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
  fi
}

# Verify image in Compute Node on GCP
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

  $GCP_CMD compute instances create "$GCP_INSTANCE_NAME" \
    --zone="$GCP_REGION-a" \
    --image-project="$GCP_PROJECT" \
    --image="$GCP_IMAGE_NAME" \
    --metadata-from-file=ssh-keys="$GCP_SSH_METADATA_FILE"
  HOST=$($GCP_CMD compute instances describe "$GCP_INSTANCE_NAME" --zone="$GCP_REGION-a" --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

  echo "⏱ Waiting for GCP instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Check if postgres is installed
  ssh -oStrictHostKeyChecking=no -i "$GCP_SSH_KEY" "$SSH_USER"@"$HOST" rpm -q postgresql
}

# Verify image in Azure
function verifyInAzure() {
  set +x
  $AZURE_CMD login --service-principal --username "${AZURE_CLIENT_ID}" --password "${AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"
  set -x

  # verify that the image exists
  $AZURE_CMD image show --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}"

  # Boot testing is currently blocked due to
  # https://github.com/Azure/azure-cli/issues/17123
  # Without this issue fixed or worked around, I'm not able to delete the disk
  # attached to the VM.
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    verifyInAWS
    ;;
  "$CLOUD_PROVIDER_GCP")
    verifyInGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    verifyInAzure
    ;;
esac

exit 0
