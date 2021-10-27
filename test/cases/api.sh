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

ARTIFACTS=ci-artifacts
mkdir -p "${ARTIFACTS}"

source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

#
# Provision the software under test.
#

/usr/libexec/osbuild-composer-test/provision.sh

#
# Set up the database queue
#
if which podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
  CONTAINER_RUNTIME=docker
else
  echo No container runtime found, install podman or docker.
  exit 2
fi

# Start the db
sudo ${CONTAINER_RUNTIME} run -d --name osbuild-composer-db \
    --health-cmd "pg_isready -U postgres -d osbuildcomposer" --health-interval 2s \
    --health-timeout 2s --health-retries 10 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=foobar \
    -e POSTGRES_DB=osbuildcomposer \
    -p 5432:5432 \
    quay.io/osbuild/postgres:13-alpine

# Dump the logs once to have a little more output
sudo ${CONTAINER_RUNTIME} logs osbuild-composer-db

# Initialize a module in a temp dir so we can get tern without introducing
# vendoring inconsistency
pushd "$(mktemp -d)"
sudo dnf install -y go
go mod init temp
go get github.com/jackc/tern
PGUSER=postgres PGPASSWORD=foobar PGDATABASE=osbuildcomposer PGHOST=localhost PGPORT=5432 \
      go run github.com/jackc/tern migrate -m /usr/share/tests/osbuild-composer/schemas
popd

cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
[koji]
allowed_domains = [ "localhost", "client.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"

[worker]
allowed_domains = [ "localhost", "worker.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
pg_host = "localhost"
pg_port = "5432"
pg_database = "osbuildcomposer"
pg_user = "postgres"
pg_password = "foobar"
pg_ssl_mode = "disable"
EOF

sudo systemctl restart osbuild-composer

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
  printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null
}

# Check that needed variables are set to access GCP.
function checkEnvGCP() {
  printenv GOOGLE_APPLICATION_CREDENTIALS GCP_BUCKET GCP_REGION GCP_API_TEST_SHARE_ACCOUNT > /dev/null
}

# Check that needed variables are set to access Azure.
function checkEnvAzure() {
  printenv AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID AZURE_RESOURCE_GROUP AZURE_LOCATION V2_AZURE_CLIENT_ID V2_AZURE_CLIENT_SECRET > /dev/null
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
    set -e
  fi
}

WORKDIR=$(mktemp -d)
KILL_PIDS=()
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

  sudo rm -rf "$WORKDIR"

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
}
trap cleanup EXIT

#
# Install the necessary cloud provider client tools
#

function installClientAWS() {
  if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -e AWS_ACCESS_KEY_ID=${V2_AWS_ACCESS_KEY_ID} \
      -e AWS_SECRET_ACCESS_KEY=${V2_AWS_SECRET_ACCESS_KEY} \
      -v ${WORKDIR}:${WORKDIR}:Z \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} aws --region $AWS_REGION --output json --color on"
  else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="aws --region $AWS_REGION --output json --color on"
  fi
  $AWS_CMD --version
}

function installClientGCP() {
  if ! hash gcloud; then
    echo "Using 'gcloud' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    # directory mounted to the container, in which gcloud stores the credentials after logging in
    GCP_CMD_CREDS_DIR="${WORKDIR}/gcloud_credentials"
    mkdir "${GCP_CMD_CREDS_DIR}"

    GCP_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -v ${GCP_CMD_CREDS_DIR}:/root/.config/gcloud:Z \
      -v ${GOOGLE_APPLICATION_CREDENTIALS}:${GOOGLE_APPLICATION_CREDENTIALS}:Z \
      -v ${WORKDIR}:${WORKDIR}:Z \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} gcloud --format=json"
  else
    echo "Using pre-installed 'gcloud' from the system"
    GCP_CMD="gcloud --format=json --quiet"
  fi
  $GCP_CMD --version
}

function installClientAzure() {
  if ! hash az; then
    echo "Using 'azure-cli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

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

case "$ID-$VERSION_ID" in
  "rhel-9.0")
    DISTRO="rhel-90"
    if [[ "$CLOUD_PROVIDER" == "$CLOUD_PROVIDER_AWS" ]]; then
      SSH_USER="ec2-user"
    else
      SSH_USER="cloud-user"
    fi
    ;;
  "rhel-8.6")
    DISTRO="rhel-86"
    if [[ "$CLOUD_PROVIDER" == "$CLOUD_PROVIDER_AWS" ]]; then
      SSH_USER="ec2-user"
    else
      SSH_USER="cloud-user"
    fi
    ;;
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
    if [[ "$CLOUD_PROVIDER" == "$CLOUD_PROVIDER_AWS" ]]; then
      SSH_USER="ec2-user"
    else
      SSH_USER="cloud-user"
    fi
    ;;
  "centos-9")
    DISTRO="centos-9"
    if [[ "$CLOUD_PROVIDER" == "$CLOUD_PROVIDER_AWS" ]]; then
      SSH_USER="ec2-user"
    else
      SSH_USER="cloud-user"
    fi
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

# generate a temp key for user tests
ssh-keygen -t rsa -f /tmp/usertest -C "usertest" -N ""

function createReqFileAWS() {
  AWS_SNAPSHOT_NAME=$(uuidgen)

  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "packages": [
      "postgresql"
    ]${SUBSCRIPTION_BLOCK},
    "users":[
      {
        "name": "user1",
        "groups": ["wheel"],
        "key": "$(cat /tmp/usertest.pub)"
      },
      {
        "name": "user2",
        "key": "$(cat /tmp/usertest.pub)"
      }
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
              "access_key_id": "${V2_AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${V2_AWS_SECRET_ACCESS_KEY}",
              "bucket": "${AWS_BUCKET}"
            },
            "ec2": {
              "access_key_id": "${V2_AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${V2_AWS_SECRET_ACCESS_KEY}",
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

#
# Global var for ostree ref (only used in aws.s3 now)
#
OSTREE_REF="test/rhel/8/edge"

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
        "ref": "${OSTREE_REF}"
      },
      "upload_request": {
          "type": "aws.s3",
          "options": {
            "region": "${AWS_REGION}",
            "s3": {
              "access_key_id": "${V2_AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${V2_AWS_SECRET_ACCESS_KEY}",
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
  AZURE_IMAGE_NAME="image-$TEST_ID"

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

function collectMetrics(){
    METRICS_OUTPUT=$(curl \
                          --cacert /etc/osbuild-composer/ca-crt.pem \
                          --key /etc/osbuild-composer/client-key.pem \
                          --cert /etc/osbuild-composer/client-crt.pem \
                          https://localhost/metrics)

    echo "$METRICS_OUTPUT" | grep "^total_compose_requests" | cut -f2 -d' '
}

function sendCompose() {
    OUTPUT=$(mktemp)
    HTTPSTATUS=$(curl \
                 --silent \
                 --show-error \
                 --cacert /etc/osbuild-composer/ca-crt.pem \
                 --key /etc/osbuild-composer/client-key.pem \
                 --cert /etc/osbuild-composer/client-crt.pem \
                 --header 'Content-Type: application/json' \
                 --request POST \
                 --data @"$REQUEST_FILE" \
                 --write-out '%{http_code}' \
                 --output "$OUTPUT" \
                 https://localhost/api/composer/v1/compose)

    test "$HTTPSTATUS" = "201"
    COMPOSE_ID=$(jq -r '.id' "$OUTPUT")
}

function waitForState() {
    local DESIRED_STATE="${1:-success}"
    local VERSION="${2:-v1}"
    local URL=https://localhost/api/composer/v1/compose/"$COMPOSE_ID"
    if [ "$VERSION" = "v2" ]; then
        URL=https://localhost/api/image-builder-composer/v2/composes/"$COMPOSE_ID"
    fi

    while true
    do
        OUTPUT=$(curl \
                     --silent \
                     --show-error \
                     --cacert /etc/osbuild-composer/ca-crt.pem \
                     --key /etc/osbuild-composer/client-key.pem \
                     --cert /etc/osbuild-composer/client-crt.pem \
                     "$URL")

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

sendCompose

# crashed/stopped/killed worker should result in a failed state
waitForState "building"
sudo systemctl stop "osbuild-worker@*"
waitForState "failure"
sudo systemctl start "osbuild-worker@1"

# full integration case
INIT_COMPOSES="$(collectMetrics)"
sendCompose
waitForState
# Same state with v2
waitForState "success" "v2"
SUBS_COMPOSES="$(collectMetrics)"

test "$UPLOAD_STATUS" = "success"
test "$UPLOAD_TYPE" = "$CLOUD_PROVIDER"
test $((INIT_COMPOSES+1)) = "$SUBS_COMPOSES"

# Make sure we get 2 job entries in the db per compose (depsolve + build)
sudo podman exec osbuild-composer-db psql -U postgres -d osbuildcomposer -c "SELECT * FROM jobs;" | grep "4 rows"

#
# Save the Manifest from the osbuild-composer store
# NOTE: The rest of the job data can contain sensitive information
#
# Suppressing shellcheck.  See https://github.com/koalaman/shellcheck/wiki/SC2024#exceptions
sudo podman exec osbuild-composer-db psql -U postgres -d osbuildcomposer -c "SELECT args->>'Manifest' FROM jobs" | sudo tee "${ARTIFACTS}/manifest.json"

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

  # Check access to user1 and user2
  check_groups=$(ssh -i /tmp/usertest "user1@$HOST" -t 'groups')
  if [[ $check_groups =~ "wheel" ]]; then
   echo "✔️  user1 has the group wheel"
  else
    echo 'user1 should have the group wheel 😢'
    exit 1
  fi
  check_groups=$(ssh -i /tmp/usertest "user2@$HOST" -t 'groups')
  if [[ $check_groups =~ "wheel" ]]; then
    echo 'user2 should not have group wheel 😢'
    exit 1
  else
   echo "✔️  user2 does not have the group wheel"
  fi
}


# Verify tarball in AWS S3
function verifyInAWSS3() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # Download the commit using the Presigned URL
  curl "${S3_URL}" --output "${WORKDIR}/edge-commit.tar"

  # extract tarball and save file list to artifacts directroy
  local COMMIT_DIR
  COMMIT_DIR="${WORKDIR}/edge-commit"
  mkdir -p "${COMMIT_DIR}"
  tar xvf "${WORKDIR}/edge-commit.tar" -C "${COMMIT_DIR}" > "${ARTIFACTS}/edge-commit-filelist.txt"

  # Verify that the commit contains the ref we defined in the request
  sudo dnf install -y ostree
  local COMMIT_REF
  COMMIT_REF=$(ostree refs --repo "${COMMIT_DIR}/repo")
  if [[ "${COMMIT_REF}" !=  "${OSTREE_REF}" ]]; then
      echo "Commit ref in archive does not match request 😠"
      exit 1
  fi

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
  TAR_COMMIT_ID=$(ostree rev-parse --repo "${COMMIT_DIR}/repo" "${OSTREE_REF}")

  if [[ "${API_COMMIT_ID}" != "${TAR_COMMIT_ID}" ]]; then
      echo "Commit ID returned from API does not match Commit ID in archive 😠"
      exit 1
  fi

  # v2 has the same result
  API_COMMIT_ID_V2=$(curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/image-builder-composer/v2/composes/"$COMPOSE_ID"/metadata | jq -r '.ostree_commit')

  if [[ "${API_COMMIT_ID_V2}" != "${TAR_COMMIT_ID}" ]]; then
      echo "Commit ID returned from API v2 does not match Commit ID in archive 😠"
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
  $AZURE_CMD login --service-principal --username "${V2_AZURE_CLIENT_ID}" --password "${V2_AZURE_CLIENT_SECRET}" --tenant "${AZURE_TENANT_ID}"
  set -x

  # verify that the image exists
  $AZURE_CMD image show --resource-group "${AZURE_RESOURCE_GROUP}" --name "${AZURE_IMAGE_NAME}"

  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  AZURE_SSH_KEY="$WORKDIR/id_azure"
  ssh-keygen -t rsa -f "$AZURE_SSH_KEY" -C "$SSH_USER" -N ""

  # Create network resources with predictable names
  $AZURE_CMD network nsg create --resource-group "$AZURE_RESOURCE_GROUP" --name "nsg-$TEST_ID" --location "$AZURE_LOCATION"
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
  $AZURE_CMD network vnet create --resource-group "$AZURE_RESOURCE_GROUP" --name "vnet-$TEST_ID" --subnet-name "snet-$TEST_ID" --location "$AZURE_LOCATION"
  $AZURE_CMD network public-ip create --resource-group "$AZURE_RESOURCE_GROUP" --name "ip-$TEST_ID" --location "$AZURE_LOCATION"
  $AZURE_CMD network nic create --resource-group "$AZURE_RESOURCE_GROUP" \
      --name "iface-$TEST_ID" \
      --subnet "snet-$TEST_ID" \
      --vnet-name "vnet-$TEST_ID" \
      --network-security-group "nsg-$TEST_ID" \
      --public-ip-address "ip-$TEST_ID" \
      --location "$AZURE_LOCATION" 

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
    --os-disk-name "disk-$TEST_ID"
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
  # Save build metadata to artifacts directory for troubleshooting
  curl --silent \
      --show-error \
      --cacert /etc/osbuild-composer/ca-crt.pem \
      --key /etc/osbuild-composer/client-key.pem \
      --cert /etc/osbuild-composer/client-crt.pem \
      https://localhost/api/composer/v1/compose/"$COMPOSE_ID"/metadata --output "${ARTIFACTS}/metadata.json"
  local PACKAGENAMES
  PACKAGENAMES=$(jq -rM '.packages[].name' "${ARTIFACTS}/metadata.json")

  if ! grep -q postgresql <<< "${PACKAGENAMES}"; then
      echo "'postgresql' not found in compose package list 😠"
      exit 1
  fi
}

verifyPackageList

#
# Make sure that requesting a non existing paquet returns a 400 error
#
REQUEST_FILE2="${WORKDIR}/request2.json"
jq '.customizations.packages = [ "jesuisunpaquetquinexistepas" ]' "$REQUEST_FILE" > "$REQUEST_FILE2"

[ "$(curl \
    --silent \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    --output /dev/null \
    --write-out '%{http_code}' \
    -H "Content-Type: application/json" \
    --data @"$REQUEST_FILE2" \
    https://localhost/api/composer/v1/compose)" = "400" ]

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
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    --output /dev/null \
    --write-out '%{http_code}' \
    -H "Content-Type: application/json" \
    --data @"$REQUEST_FILE2" \
    https://localhost/api/composer/v1/compose)" = "500" ]

sudo mv -f /usr/libexec/osbuild-composer/dnf-json.bak /usr/libexec/osbuild-composer/dnf-json

#
# Verify oauth2
#
cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
[koji]
enable_tls = false
enable_mtls = false
enable_jwt = true
jwt_keys_url = "https://localhost:8080/certs"
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
jwt_acl_file = ""
[worker]
pg_host = "localhost"
pg_port = "5432"
pg_database = "osbuildcomposer"
pg_user = "postgres"
pg_password = "foobar"
pg_ssl_mode = "disable"
enable_tls = true
enable_mtls = false
enable_jwt = true
jwt_keys_url = "https://localhost:8080/certs"
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
EOF

cat <<EOF | sudo tee "/etc/osbuild-worker/token"
offlineToken
EOF

cat <<EOF | sudo tee "/etc/osbuild-worker/osbuild-worker.toml"
[authentication]
oauth_url = http://localhost:8081/token
offline_token = "/etc/osbuild-worker/token"
EOF

# Spin up an https instance for the composer-api and worker-api; the auth handler needs to hit an ssl `/certs` endpoint
sudo /usr/libexec/osbuild-composer-test/osbuild-mock-openid-provider -rsaPubPem /etc/osbuild-composer/client-crt.pem -rsaPem /etc/osbuild-composer/client-key.pem -cert /etc/osbuild-composer/composer-crt.pem -key /etc/osbuild-composer/composer-key.pem &
KILL_PIDS+=("$!")
# Spin up an http instance for the worker client to bypass the need to specify an extra CA
sudo /usr/libexec/osbuild-composer-test/osbuild-mock-openid-provider -a localhost:8081 -rsaPubPem /etc/osbuild-composer/client-crt.pem -rsaPem /etc/osbuild-composer/client-key.pem &
KILL_PIDS+=("$!")

sudo systemctl restart osbuild-composer

until curl --output /dev/null --silent --fail localhost:8081/token; do
    sleep 0.5
done
TOKEN="$(curl localhost:8081/token | jq -r .access_token)"

[ "$(curl \
        --silent \
        --output /dev/null \
        --write-out '%{http_code}' \
        --header "Authorization: Bearer $TOKEN" \
        http://localhost:443/api/composer/v1/version)" = "200" ]

[ "$(curl \
        --silent \
        --output /dev/null \
        --write-out '%{http_code}' \
        --header "Authorization: Bearer badtoken" \
        http://localhost:443/api/composer/v1/version)" = "401" ]

sudo systemctl start osbuild-remote-worker@localhost:8700.service
sudo systemctl is-active --quiet osbuild-remote-worker@localhost:8700.service

exit 0
