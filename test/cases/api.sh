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

#
# Cloud provider / target names
#

CLOUD_PROVIDER_AWS="aws"
CLOUD_PROVIDER_GCP="gcp"
CLOUD_PROVIDER_AZURE="azure"
CLOUD_PROVIDER_AWS_S3="aws.s3"
CLOUD_PROVIDER_GENERIC_S3="generic.s3"

#
# Supported Image type names
#
IMAGE_TYPE_AWS="aws"
IMAGE_TYPE_AZURE="azure"
IMAGE_TYPE_EDGE_COMMIT="edge-commit"
IMAGE_TYPE_EDGE_CONTAINER="edge-container"
IMAGE_TYPE_EDGE_INSTALLER="edge-installer"
IMAGE_TYPE_GCP="gcp"
IMAGE_TYPE_IMAGE_INSTALLER="image-installer"
IMAGE_TYPE_GUEST="guest-image"
IMAGE_TYPE_VSPHERE="vsphere"

if (( $# > 2 )); then
    echo "$0 does not support more than two arguments"
    exit 1
fi

if (( $# == 0 )); then
    echo "$0 requires that you set the image type to build"
    exit 1
fi

set -euxo pipefail

IMAGE_TYPE="$1"

# select cloud provider based on image type
#
# the supported image types are listed in the api spec (internal/cloudapi/v2/openapi.v2.yml)
case ${IMAGE_TYPE} in
    "$IMAGE_TYPE_AWS")
        CLOUD_PROVIDER="${CLOUD_PROVIDER_AWS}"
        ;;
    "$IMAGE_TYPE_AZURE")
        CLOUD_PROVIDER="${CLOUD_PROVIDER_AZURE}"
        ;;
    "$IMAGE_TYPE_GCP")
        CLOUD_PROVIDER="${CLOUD_PROVIDER_GCP}"
        ;;
    "$IMAGE_TYPE_EDGE_COMMIT"|"$IMAGE_TYPE_EDGE_CONTAINER"|"$IMAGE_TYPE_EDGE_INSTALLER"|"$IMAGE_TYPE_IMAGE_INSTALLER"|"$IMAGE_TYPE_GUEST"|"$IMAGE_TYPE_VSPHERE")
        # blobby image types: upload to s3 and provide download link
        CLOUD_PROVIDER="${2:-$CLOUD_PROVIDER_AWS_S3}"
        if [ "${CLOUD_PROVIDER}" != "${CLOUD_PROVIDER_AWS_S3}" ] && [ "${CLOUD_PROVIDER}" != "${CLOUD_PROVIDER_GENERIC_S3}" ]; then
            echo "${IMAGE_TYPE} can only be uploaded to either ${CLOUD_PROVIDER_AWS_S3} or ${CLOUD_PROVIDER_GENERIC_S3}"
            exit 1
        fi
        ;;
    *)
        echo "Unknown image type: ${IMAGE_TYPE}"
        exit 1
esac

# Colorful timestamped output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

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
DB_CONTAINER_NAME="osbuild-composer-db"
sudo ${CONTAINER_RUNTIME} run -d --name "${DB_CONTAINER_NAME}" \
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
log_level = "debug"
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
pg_max_conns = 10
EOF

sudo systemctl restart osbuild-composer

greenprint "Using Cloud Provider / Target ${CLOUD_PROVIDER} for Image Type ${IMAGE_TYPE}"

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
  printenv API_TEST_SUBSCRIPTION_ORG_ID API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2 > /dev/null
}

function checkEnvVSphere() {
  printenv GOVMOMI_USERNAME GOVMOMI_PASSWORD GOVMOMI_URL GOVMOMI_CLUSTER GOVC_DATACENTER GOVMOMI_DATASTORE GOVMOMI_FOLDER GOVMOMI_NETWORK  > /dev/null
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS" | "$CLOUD_PROVIDER_AWS_S3")
    checkEnvAWS
    [[ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]] && checkEnvVSphere
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
    $AWS_CMD ec2 terminate-instances --instance-ids "$AWS_INSTANCE_ID"
    $AWS_CMD ec2 deregister-image --image-id "$AMI_IMAGE_ID"
    $AWS_CMD ec2 delete-snapshot --snapshot-id "$AWS_SNAPSHOT_ID"
    $AWS_CMD ec2 delete-key-pair --key-name "key-for-$AMI_IMAGE_ID"
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
    $AWS_CMD s3 rm "${S3_URI}"
  fi
}

function cleanupGCP() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  GCP_CMD="${GCP_CMD:-}"
  GCP_IMAGE_NAME="${GCP_IMAGE_NAME:-}"
  GCP_INSTANCE_NAME="${GCP_INSTANCE_NAME:-}"
  GCP_ZONE="${GCP_ZONE:-}"

  if [ -n "$GCP_CMD" ]; then
    $GCP_CMD compute instances delete --zone="$GCP_ZONE" "$GCP_INSTANCE_NAME"
    $GCP_CMD compute images delete "$GCP_IMAGE_NAME"
  fi
}

function cleanupAzure() {
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

function cleanupVSphere() {
    # since this function can be called at any time, ensure that we don't expand unbound variables
    GOVC_CMD="${GOVC_CMD:-}"
    VSPHERE_VM_NAME="${VSPHERE_VM_NAME:-}"
    VSPHERE_CIDATA_ISO_PATH="${VSPHERE_CIDATA_ISO_PATH:-}"

    greenprint "🧹 Cleaning up the VSphere VM"
    $GOVC_CMD vm.destroy \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        "${VSPHERE_VM_NAME}"

    greenprint "🧹 Cleaning up the VSphere Datastore"
    $GOVC_CMD datastore.rm \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        -f \
        "${VSPHERE_CIDATA_ISO_PATH}"

    $GOVC_CMD datastore.rm \
        -u "${GOVMOMI_USERNAME}:${GOVMOMI_PASSWORD}@${GOVMOMI_URL}" \
        -k=true \
        -dc="${GOVC_DATACENTER}" \
        -ds="${GOVMOMI_DATASTORE}" \
        -f \
        "${VSPHERE_VM_NAME}"
}

function cleanupGenericS3() {
  MINIO_CONTAINER_NAME="${MINIO_CONTAINER_NAME:-}"
  if [ -n "${MINIO_CONTAINER_NAME}" ]; then
    sudo ${CONTAINER_RUNTIME} kill "${MINIO_CONTAINER_NAME}"
  fi
}

function dump_db() {
  # Disable -x for these commands to avoid printing the whole result and manifest into the log
  set +x

  # Make sure we get 3 job entries in the db per compose (depsolve + manifest + build)
  sudo ${CONTAINER_RUNTIME} exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c "SELECT * FROM jobs;" | grep "9 rows" > /dev/null

  # Save the result, including the manifest, for the job, straight from the db
  sudo ${CONTAINER_RUNTIME} exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c "SELECT result FROM jobs WHERE type='manifest-id-only'" \
    | gpg --batch --yes --passphrase "${GPG_SYMMETRIC_PASSPHRASE}" -o "${ARTIFACTS}/build-result.gpg" --symmetric -
  set -x
}

WORKDIR=$(mktemp -d)
KILL_PIDS=()
function cleanup() {
  greenprint "== Script execution stopped or finished - Cleaning up =="
  set +eu
  case $CLOUD_PROVIDER in
    "$CLOUD_PROVIDER_AWS")
      cleanupAWS
      ;;
    "$CLOUD_PROVIDER_AWS_S3")
      cleanupAWSS3
      [[ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]] && cleanupVSphere
      ;;
    "$CLOUD_PROVIDER_GCP")
      cleanupGCP
      ;;
    "$CLOUD_PROVIDER_AZURE")
      cleanupAzure
      ;;
    "$CLOUD_PROVIDER_GENERIC_S3")
      cleanupGenericS3
      [[ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]] && cleanupVSphere
    ;;
  esac

  # dump the DB here to ensure that it gets dumped even if the test fails
  dump_db

  sudo ${CONTAINER_RUNTIME} kill "${DB_CONTAINER_NAME}"
  sudo ${CONTAINER_RUNTIME} rm "${DB_CONTAINER_NAME}"

  sudo rm -rf "$WORKDIR"

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
  set -eu
}
trap cleanup EXIT

# make a dummy rpm and repo to test payload_repositories
sudo dnf install -y rpm-build createrepo
DUMMYRPMDIR=$(mktemp -d)
DUMMYSPECFILE="$DUMMYRPMDIR/dummy.spec"
PAYLOAD_REPO_PORT="9999"
PAYLOAD_REPO_URL="http://localhost:9999"
pushd "$DUMMYRPMDIR"

cat <<EOF > "$DUMMYSPECFILE"
#----------- spec file starts ---------------
Name:                   dummy
Version:                1.0.0
Release:                0
BuildArch:              noarch
Vendor:                 dummy
Summary:                Provides %{name}
License:                BSD
Provides:               dummy

%description
%{summary}

%files
EOF

mkdir -p "DUMMYRPMDIR/rpmbuild"
rpmbuild --quiet --define "_topdir $DUMMYRPMDIR/rpmbuild" -bb "$DUMMYSPECFILE"

mkdir -p "$DUMMYRPMDIR/repo"
cp "$DUMMYRPMDIR"/rpmbuild/RPMS/noarch/*rpm "$DUMMYRPMDIR/repo"
pushd "$DUMMYRPMDIR/repo"
createrepo .
sudo python3 -m http.server "$PAYLOAD_REPO_PORT" &
KILL_PIDS+=("$!")
popd
popd


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

function installClientVSphere() {
    if ! hash govc; then
        greenprint "Installing govc"
        pushd "${WORKDIR}"
            curl -Ls --retry 5 --output govc.gz \
                https://github.com/vmware/govmomi/releases/download/v0.24.0/govc_linux_amd64.gz
            gunzip -f govc.gz
            GOVC_CMD="${WORKDIR}/govc"
            chmod +x "${GOVC_CMD}"
        popd
    else
        echo "Using pre-installed 'govc' from the system"
        GOVC_CMD="govc"
    fi

    $GOVC_CMD version
}

function installGenericS3() {
  local CONTAINER_MINIO_SERVER="quay.io/minio/minio:latest"
  MINIO_CONTAINER_NAME="minio-server"
  MINIO_ENDPOINT="http://localhost:9000"
  local MINIO_ROOT_USER="X29DU5Q6C5NKDQ8PLGVT"
  local MINIO_ROOT_PASSWORD
  MINIO_ROOT_PASSWORD=$(date +%s | sha256sum | base64 | head -c 32 ; echo)
  MINIO_BUCKET="ci-test"
  local MINIO_REGION="us-east-1"
  local MINIO_CREDENTIALS_FILE="/etc/osbuild-worker/minio-creds"

  sudo ${CONTAINER_RUNTIME} run --rm -d \
    --name ${MINIO_CONTAINER_NAME} \
    -p 9000:9000 \
    -e MINIO_BROWSER=off \
    -e MINIO_ROOT_USER="${MINIO_ROOT_USER}" \
    -e MINIO_ROOT_PASSWORD="${MINIO_ROOT_PASSWORD}" \
    ${CONTAINER_MINIO_SERVER} server /data

  if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo ${CONTAINER_RUNTIME} pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -e AWS_ACCESS_KEY_ID=${MINIO_ROOT_USER} \
      -e AWS_SECRET_ACCESS_KEY=${MINIO_ROOT_PASSWORD} \
      -v ${WORKDIR}:${WORKDIR}:Z \
      --network host \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} aws"
  else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="AWS_ACCESS_KEY_ID=${MINIO_ROOT_USER} \
      AWS_SECRET_ACCESS_KEY=${MINIO_ROOT_PASSWORD} \
      aws"
  fi
  AWS_CMD+=" --region $MINIO_REGION --output json --color on --endpoint-url $MINIO_ENDPOINT"
  $AWS_CMD --version

  # Configure the local server (retry until the service is up)
  MINIO_CONFIGURE_RETRY=0
  MINIO_CONFIGURE_MAX_RETRY=5
  MINIO_RETRY_INTERVAL=15
  until [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]
  do
      ${AWS_CMD} s3 ls && break
      MINIO_CONFIGURE_RETRY=$((MINIO_CONFIGURE_RETRY + 1))
    echo "Retrying [${MINIO_CONFIGURE_RETRY}/${MINIO_CONFIGURE_MAX_RETRY}] in ${MINIO_RETRY_INTERVAL}(s) "
    sleep ${MINIO_RETRY_INTERVAL}
  done

  if [ "${MINIO_CONFIGURE_RETRY}" -ge "${MINIO_CONFIGURE_MAX_RETRY}" ]; then
    echo "Failed to communicate with the MinIO server after ${MINIO_CONFIGURE_MAX_RETRY} attempts!"
    exit 1
  fi

  # Create the bucket
  ${AWS_CMD} s3 mb s3://${MINIO_BUCKET}

  cat <<EOF | sudo tee "${MINIO_CREDENTIALS_FILE}"
[default]
aws_access_key_id = ${MINIO_ROOT_USER}
aws_secret_access_key = ${MINIO_ROOT_PASSWORD}
EOF

  cat <<EOF | sudo tee "/etc/osbuild-worker/osbuild-worker.toml"
[generic_s3]
credentials = "${MINIO_CREDENTIALS_FILE}"
endpoint = "${MINIO_ENDPOINT}"
region = "${MINIO_REGION}"
bucket = "${MINIO_BUCKET}"
EOF

  sudo systemctl restart "osbuild-worker@1"
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS" | "$CLOUD_PROVIDER_AWS_S3")
    installClientAWS
    [[ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]] && installClientVSphere
    ;;
  "$CLOUD_PROVIDER_GCP")
    installClientGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    installClientAzure
    ;;
  "$CLOUD_PROVIDER_GENERIC_S3")
    installGenericS3
    [[ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]] && installClientVSphere
    ;;
esac

#
# Make sure /openapi and endpoints return success
#

curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/image-builder-composer/v2/openapi | jq .

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

if [[ "$ID" == "fedora" ]]; then
  # fedora uses fedora for everything
  SSH_USER="fedora"
elif [[ "$CLOUD_PROVIDER" == "$CLOUD_PROVIDER_AWS" ]]; then
  # RHEL and centos use ec2-user for AWS
  SSH_USER="ec2-user"
else
  # RHEL and centos use cloud-user for other clouds
  SSH_USER="cloud-user"
fi

# This removes dot from VERSION_ID.
# ID == rhel   && VERSION_ID == 8.6 => DISTRO == rhel-86
# ID == centos && VERSION_ID == 8   => DISTRO == centos-8
# ID == fedora && VERSION_ID == 35  => DISTRO == fedora-35
DISTRO="$ID-${VERSION_ID//./}"

# Only RHEL need subscription block.
if [[ "$ID" == "rhel" ]]; then
  SUBSCRIPTION_BLOCK=$(cat <<EndOfMessage
,
    "subscription": {
      "organization": "${API_TEST_SUBSCRIPTION_ORG_ID:-}",
      "activation_key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2:-}",
      "base_url": "https://cdn.redhat.com/",
      "server_url": "subscription.rhsm.redhat.com",
      "insights": true
    }
EndOfMessage
)
else
  SUBSCRIPTION_BLOCK=''
fi

# generate a temp key for user tests
ssh-keygen -t rsa-sha2-512 -f "${WORKDIR}/usertest" -C "usertest" -N ""

function createReqFileAWS() {
  AWS_SNAPSHOT_NAME=${TEST_ID}

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
      "repositories": $(jq ".\"$ARCH\" | .[] | select((has(\"image_type_tags\") | not) or (.\"image_type_tags\" | index(\"${IMAGE_TYPE}\")))" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json | jq -s .),
      "upload_options": {
        "region": "${AWS_REGION}",
        "snapshot_name": "${AWS_SNAPSHOT_NAME}",
        "share_with_accounts": ["${AWS_API_TEST_SHARE_ACCOUNT}"]
    }
  }
}
EOF
}

#
# Global var for ostree ref (only used in aws.s3 now)
#
OSTREE_REF="test/rhel/8/edge"

function createReqFileS3() {
  local IMAGE_REQUEST_REGION=${1:-""}
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
    "repositories": $(jq ".\"$ARCH\" | .[] | select((has(\"image_type_tags\") | not) or (.\"image_type_tags\" | index(\"${IMAGE_TYPE}\")))" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json | jq -s .),
    "ostree": {
      "ref": "${OSTREE_REF}"
    },
    "upload_options": {
      "region": "${IMAGE_REQUEST_REGION}"
    }
  }
}
EOF
}

# the VSphere test case does not create any additional users,
# since this is not supported by the service UI
function createReqFileS3VSphere() {
  local IMAGE_REQUEST_REGION=${1:-""}
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
      "region": "${IMAGE_REQUEST_REGION}"
    }
  }
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
    ]${SUBSCRIPTION_BLOCK}
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\" | .[] | select((has(\"image_type_tags\") | not) or (.\"image_type_tags\" | index(\"${IMAGE_TYPE}\")))" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json | jq -s .),
    "upload_options": {
      "bucket": "${GCP_BUCKET}",
      "region": "${GCP_REGION}",
      "image_name": "${GCP_IMAGE_NAME}",
      "share_with_accounts": ["${GCP_API_TEST_SHARE_ACCOUNT}"]
    }
  }
}
EOF
}

function createReqFileAzure() {
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
    ]${SUBSCRIPTION_BLOCK}
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\" | .[] | select((has(\"image_type_tags\") | not) or (.\"image_type_tags\" | index(\"${IMAGE_TYPE}\")))" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json | jq -s .),
    "upload_options": {
      "tenant_id": "${AZURE_TENANT_ID}",
      "subscription_id": "${AZURE_SUBSCRIPTION_ID}",
      "resource_group": "${AZURE_RESOURCE_GROUP}",
      "location": "${AZURE_LOCATION}",
      "image_name": "${AZURE_IMAGE_NAME}"
    }
  }
}
EOF
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    createReqFileAWS
    ;;
  "$CLOUD_PROVIDER_AWS_S3")
    if [[ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]]; then
      createReqFileS3VSphere "${AWS_REGION}"
    else
      createReqFileS3 "${AWS_REGION}"
    fi
    ;;
  "$CLOUD_PROVIDER_GCP")
    createReqFileGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    createReqFileAzure
    ;;
  "$CLOUD_PROVIDER_GENERIC_S3")
    if [[ "${IMAGE_TYPE}" == "${IMAGE_TYPE_VSPHERE}" ]]; then
      createReqFileS3VSphere
    else
      createReqFileS3
    fi
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

    echo "$METRICS_OUTPUT" | grep "^image_builder_composer_total_compose_requests" | cut -f2 -d' '
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
                 --data @"$1" \
                 --write-out '%{http_code}' \
                 --output "$OUTPUT" \
                 https://localhost/api/image-builder-composer/v2/compose)

    test "$HTTPSTATUS" = "201"
    COMPOSE_ID=$(jq -r '.id' "$OUTPUT")
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
                     "https://localhost/api/image-builder-composer/v2/composes/$COMPOSE_ID")

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

#
# Make sure that requesting a non existing paquet results in failure
#
REQUEST_FILE2="${WORKDIR}/request2.json"
jq '.customizations.packages = [ "jesuisunpaquetquinexistepas" ]' "$REQUEST_FILE" > "$REQUEST_FILE2"

sendCompose "$REQUEST_FILE2"
waitForState "failure"

# crashed/stopped/killed worker should result in a failed state
sendCompose "$REQUEST_FILE"
waitForState "building"
sudo systemctl stop "osbuild-worker@*"
waitForState "failure"
sudo systemctl start "osbuild-worker@1"

# full integration case
INIT_COMPOSES="$(collectMetrics)"
sendCompose "$REQUEST_FILE"
waitForState
SUBS_COMPOSES="$(collectMetrics)"

test "$UPLOAD_STATUS" = "success"
EXPECTED_UPLOAD_TYPE="$CLOUD_PROVIDER"
if [ "${CLOUD_PROVIDER}" == "${CLOUD_PROVIDER_GENERIC_S3}" ]; then
  EXPECTED_UPLOAD_TYPE="${CLOUD_PROVIDER_AWS_S3}"
fi
test "$UPLOAD_TYPE" = "$EXPECTED_UPLOAD_TYPE"
test $((INIT_COMPOSES+1)) = "$SUBS_COMPOSES"

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

function checkUploadStatusOptionsGenericS3() {
  local S3_URL
  S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  # S3 URL contains endpoint and bucket name
  echo "$S3_URL" | grep -F "$MINIO_ENDPOINT" -
  echo "$S3_URL" | grep -F "$MINIO_BUCKET" -
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
  "$CLOUD_PROVIDER_GENERIC_S3")
    checkUploadStatusOptionsGenericS3
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
          ssh-keyscan "$HOST" | sudo tee -a /root/.ssh/known_hosts
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
  $_ssh rpm -q postgresql dummy

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

# Create a cloud-int user-data file
#
# Returns:
#   - path to the user-data file
#
# Arguments:
#   $1 - default username
#   $2 - path to the SSH public key to set as authorized for the user
function _createCIUserdata() {
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
function _createCIMetadata() {
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
function _createCIUserdataISO() {
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

# Verify image in EC2 on AWS
function verifyInAWS() {
  $AWS_CMD ec2 describe-images \
    --owners self \
    --filters Name=name,Values="$AWS_SNAPSHOT_NAME" \
    > "$WORKDIR/ami.json"

  AMI_IMAGE_ID=$(jq -r '.Images[].ImageId' "$WORKDIR/ami.json")
  AWS_SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' "$WORKDIR/ami.json")

  # Tag image and snapshot with "gitlab-ci-test" tag
  $AWS_CMD ec2 create-tags \
    --resources "${AWS_SNAPSHOT_ID}" "${AMI_IMAGE_ID}" \
    --tags Key=gitlab-ci-test,Value=true


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
  $AWS_CMD ec2 run-instances --image-id "$AMI_IMAGE_ID" --count 1 --instance-type t2.micro --key-name "key-for-$AMI_IMAGE_ID" --tag-specifications 'ResourceType=instance,Tags=[{Key=gitlab-ci-test,Value=true}]' > "$WORKDIR/instances.json"
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
  check_groups=$(ssh -oStrictHostKeyChecking=no -i "${WORKDIR}/usertest" "user1@$HOST" -t 'groups')
  if [[ $check_groups =~ "wheel" ]]; then
   echo "✔️  user1 has the group wheel"
  else
    echo 'user1 should have the group wheel 😢'
    exit 1
  fi
  check_groups=$(ssh -oStrictHostKeyChecking=no -i "${WORKDIR}/usertest" "user2@$HOST" -t 'groups')
  if [[ $check_groups =~ "wheel" ]]; then
    echo 'user2 should not have group wheel 😢'
    exit 1
  else
   echo "✔️  user2 does not have the group wheel"
  fi
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
    _ci_userdata_path="$(_createCIUserdata "${SSH_USER}" "${_vsphere_ssh_key}.pub")"
    local _ci_metadata_path
    _ci_metadata_path="$(_createCIMetadata "${VSPHERE_VM_NAME}")"
    greenprint "💿 Creating cloud-init user-data ISO"
    local _ci_iso_path
    _ci_iso_path="$(_createCIUserdataISO "${_ci_userdata_path}" "${_ci_metadata_path}")"

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

# Verify s3 blobs
function verifyInS3() {
    local BUCKET_NAME=${1}
    local S3_URL
    S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
    greenprint "Verifying S3 object at ${S3_URL}"

    # Tag the resource as a test file
    local S3_FILENAME
    S3_FILENAME=$(echo "${S3_URL}" | grep -oP '(?<=/)[^/]+(?=\?)')

    # tag the object, also verifying that it exists in the bucket as expected
    $AWS_CMD s3api put-object-tagging \
        --bucket "${BUCKET_NAME}" \
        --key "${S3_FILENAME}" \
        --tagging '{"TagSet": [{ "Key": "gitlab-ci-test", "Value": "true" }]}'

    greenprint "✅ Successfully tagged S3 object"

    # Download the object using the Presigned URL and inspect
    case ${IMAGE_TYPE} in
        "$IMAGE_TYPE_EDGE_COMMIT")
            curl "${S3_URL}" --output "${WORKDIR}/edge-commit.tar"
            verifyEdgeCommit "${WORKDIR}/edge-commit.tar"
            ;;
        "${IMAGE_TYPE_GUEST}")
            curl "${S3_URL}" --output "${WORKDIR}/disk.qcow2"
            verifyDisk "${WORKDIR}/disk.qcow2"
            ;;

        "${IMAGE_TYPE_VSPHERE}")
            curl "${S3_URL}" --output "${WORKDIR}/disk.vmdk"
            verifyInVSphere "${WORKDIR}/disk.vmdk"
            ;;
        *)
            greenprint "No validation method for image type ${IMAGE_TYPE}"
            ;;
    esac

    greenprint "✅ Successfully verified S3 object"
}

# Verify image in Compute Engine on GCP
function verifyInGCP() {
  # Authenticate
  $GCP_CMD auth activate-service-account --key-file "$GOOGLE_APPLICATION_CREDENTIALS"
  # Extract and set the default project to be used for commands
  GCP_PROJECT=$(jq -r '.project_id' "$GOOGLE_APPLICATION_CREDENTIALS")
  $GCP_CMD config set project "$GCP_PROJECT"

  # Add "gitlab-ci-test" label to the image
  $GCP_CMD compute images add-labels "$GCP_IMAGE_NAME" --labels=gitlab-ci-test=true

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
  ssh-keygen -t rsa-sha2-512 -f "$GCP_SSH_KEY" -C "$SSH_USER" -N ""

  # create the instance
  # resource ID can have max 62 characters, the $GCP_TEST_ID_HASH contains 56 characters
  GCP_INSTANCE_NAME="vm-$GCP_TEST_ID_HASH"

  # Ensure that we use random GCP region with available 'IN_USE_ADDRESSES' quota
  # We use the CI variable "GCP_REGION" as the base for expression to filter regions.
  # It works best if the "GCP_REGION" is set to a storage multi-region, such as "us"
  local GCP_COMPUTE_REGION
  GCP_COMPUTE_REGION=$($GCP_CMD compute regions list --filter="name:$GCP_REGION* AND status=UP" | jq -r '.[] | select(.quotas[] as $quota | $quota.metric == "IN_USE_ADDRESSES" and $quota.limit > $quota.usage) | .name' | shuf -n1)

  # Randomize the used GCP zone to prevent hitting "exhausted resources" error on each test re-run
  GCP_ZONE=$($GCP_CMD compute zones list --filter="region=$GCP_COMPUTE_REGION AND status=UP" | jq -r '.[].name' | shuf -n1)

  # Pick the smallest '^n\d-standard-\d$' machine type from those available in the zone
  local GCP_MACHINE_TYPE
  GCP_MACHINE_TYPE=$($GCP_CMD compute machine-types list --filter="zone=$GCP_ZONE AND name~^n\d-standard-\d$" | jq -r '.[].name' | sort | head -1)

  $GCP_CMD compute instances create "$GCP_INSTANCE_NAME" \
    --zone="$GCP_ZONE" \
    --image-project="$GCP_PROJECT" \
    --image="$GCP_IMAGE_NAME" \
    --machine-type="$GCP_MACHINE_TYPE" \
    --labels=gitlab-ci-test=true

  HOST=$($GCP_CMD compute instances describe "$GCP_INSTANCE_NAME" --zone="$GCP_ZONE" --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

  echo "⏱ Waiting for GCP instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Verify image
  _ssh="$GCP_CMD compute ssh --strict-host-key-checking=no --ssh-key-file=$GCP_SSH_KEY --zone=$GCP_ZONE --quiet $SSH_USER@$GCP_INSTANCE_NAME --"
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
  ssh-keygen -t rsa-sha2-512 -f "$AZURE_SSH_KEY" -C "$SSH_USER" -N ""

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
    verifyInS3 "${AWS_BUCKET}"
    ;;
  "$CLOUD_PROVIDER_GCP")
    verifyInGCP
    ;;
  "$CLOUD_PROVIDER_AZURE")
    verifyInAzure
    ;;
  "$CLOUD_PROVIDER_GENERIC_S3")
    verifyInS3 "${MINIO_BUCKET}"
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
      https://localhost/api/image-builder-composer/v2/composes/"$COMPOSE_ID"/metadata --output "${ARTIFACTS}/metadata.json"
  local PACKAGENAMES
  PACKAGENAMES=$(jq -rM '.packages[].name' "${ARTIFACTS}/metadata.json")

  if ! grep -q postgresql <<< "${PACKAGENAMES}"; then
      echo "'postgresql' not found in compose package list 😠"
      exit 1
  fi
}

verifyPackageList

#
# Verify oauth2
#
cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
[koji]
enable_tls = false
enable_mtls = false
enable_jwt = true
jwt_keys_urls = ["https://localhost:8080/certs"]
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
jwt_acl_file = ""
jwt_tenant_provider_fields = ["rh-org-id"]
[worker]
pg_host = "localhost"
pg_port = "5432"
enable_artifacts = false
pg_database = "osbuildcomposer"
pg_user = "postgres"
pg_password = "foobar"
pg_ssl_mode = "disable"
enable_tls = true
enable_mtls = false
enable_jwt = true
jwt_keys_urls = ["https://localhost:8080/certs"]
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
jwt_tenant_provider_fields = ["rh-org-id"]
EOF

REFRESH_TOKEN="offlineToken"
cat <<EOF | sudo tee "/etc/osbuild-worker/token"
$REFRESH_TOKEN
EOF

cat <<EOF | sudo tee "/etc/osbuild-worker/osbuild-worker.toml"
[authentication]
oauth_url = http://localhost:8081/token
client_id = "rhsm-api"
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

TOKEN="$(curl --request POST \
        --data "refresh_token=$REFRESH_TOKEN" \
        --header "Content-Type: application/x-www-form-urlencoded" \
        --silent \
        --show-error \
        --fail \
        localhost:8081/token | jq -r .access_token)"

[ "$(curl \
        --silent \
        --output /dev/null \
        --write-out '%{http_code}' \
        --header "Authorization: Bearer $TOKEN" \
        http://localhost:443/api/image-builder-composer/v2/openapi)" = "200" ]

# /openapi doesn't need auth
[ "$(curl \
        --silent \
        --output /dev/null \
        --write-out '%{http_code}' \
        --header "Authorization: Bearer badtoken" \
        http://localhost:443/api/image-builder-composer/v2/openapi)" = "200" ]


# /composes/$ID does need auth
[ "$(curl \
        --silent \
        --output /dev/null \
        --write-out '%{http_code}' \
        --header "Authorization: Bearer badtoken" \
        http://localhost:443/api/image-builder-composer/v2/composes/"$COMPOSE_ID")" = "401" ]


sudo systemctl start osbuild-remote-worker@localhost:8700.service
sudo systemctl is-active --quiet osbuild-remote-worker@localhost:8700.service

exit 0
