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
CLOUD_PROVIDER_CONTAINER_IMAGE_REGISTRY="container"

#
# Supported Image type names
#
export IMAGE_TYPE_AWS="aws"
export IMAGE_TYPE_AZURE="azure"
export IMAGE_TYPE_EDGE_COMMIT="edge-commit"
export IMAGE_TYPE_EDGE_CONTAINER="edge-container"
export IMAGE_TYPE_EDGE_INSTALLER="edge-installer"
export IMAGE_TYPE_GCP="gcp"
export IMAGE_TYPE_IMAGE_INSTALLER="image-installer"
export IMAGE_TYPE_GUEST="guest-image"
export IMAGE_TYPE_VSPHERE="vsphere"
export IMAGE_TYPE_IOT_COMMIT="iot-commit"

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
    "$IMAGE_TYPE_EDGE_CONTAINER")
        CLOUD_PROVIDER="${CLOUD_PROVIDER_CONTAINER_IMAGE_REGISTRY}"
        ;;
    "$IMAGE_TYPE_EDGE_COMMIT"|"$IMAGE_TYPE_IOT_COMMIT"|"$IMAGE_TYPE_EDGE_INSTALLER"|"$IMAGE_TYPE_IMAGE_INSTALLER"|"$IMAGE_TYPE_GUEST"|"$IMAGE_TYPE_VSPHERE")
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


ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Container image used for cloud provider CLI tools
export CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

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
sudo "${CONTAINER_RUNTIME}" run -d --name "${DB_CONTAINER_NAME}" \
    --health-cmd "pg_isready -U postgres -d osbuildcomposer" --health-interval 2s \
    --health-timeout 2s --health-retries 10 \
    -e POSTGRES_USER=postgres \
    -e POSTGRES_PASSWORD=foobar \
    -e POSTGRES_DB=osbuildcomposer \
    -p 5432:5432 \
    quay.io/osbuild/postgres:13-alpine

# Dump the logs once to have a little more output
sudo "${CONTAINER_RUNTIME}" logs osbuild-composer-db

# Initialize a module in a temp dir so we can get tern without introducing
# vendoring inconsistency
pushd "$(mktemp -d)"
sudo dnf install -y go
go mod init temp
go install github.com/jackc/tern@latest
PGUSER=postgres PGPASSWORD=foobar PGDATABASE=osbuildcomposer PGHOST=localhost PGPORT=5432 \
    "$(go env GOPATH)"/bin/tern migrate -m /usr/share/tests/osbuild-composer/schemas
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


# Load a correct test runner.
# Each one must define following methods:
# - checkEnv
# - cleanup
# - createReqFile
# - installClient
# - checkUploadStatusOptions
case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    source /usr/libexec/tests/osbuild-composer/api/aws.sh
    ;;
  "$CLOUD_PROVIDER_AWS_S3")
    source /usr/libexec/tests/osbuild-composer/api/aws.s3.sh
    ;;
  "$CLOUD_PROVIDER_GENERIC_S3")
    source /usr/libexec/tests/osbuild-composer/api/generic.s3.sh
    ;;
  "$CLOUD_PROVIDER_GCP")
    source /usr/libexec/tests/osbuild-composer/api/gcp.sh
    ;;
  "$CLOUD_PROVIDER_AZURE")
    source /usr/libexec/tests/osbuild-composer/api/azure.sh
    ;;
  "$CLOUD_PROVIDER_CONTAINER_IMAGE_REGISTRY")
    source /usr/libexec/tests/osbuild-composer/api/container.registry.sh
    ;;
  *)
    echo "Unknown cloud provider: ${CLOUD_PROVIDER}"
    exit 1
esac

# Verify that this script is running in the right environment.
checkEnv
# Check that needed variables are set to register to RHSM (RHEL only)
[[ "$ID" == "rhel" ]] && printenv API_TEST_SUBSCRIPTION_ORG_ID API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2 > /dev/null

function dump_db() {
  # Disable -x for these commands to avoid printing the whole result and manifest into the log
  set +x

  # Save the result, including the manifest, for the job, straight from the db
  sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c "SELECT result FROM jobs WHERE type='manifest-id-only'" \
    | sudo tee "${ARTIFACTS}/build-result.txt"
  set -x
}

WORKDIR=$(mktemp -d)
KILL_PIDS=()
function cleanups() {
  set +eu

  cleanup

  # dump the DB here to ensure that it gets dumped even if the test fails
  dump_db

  sudo "${CONTAINER_RUNTIME}" kill "${DB_CONTAINER_NAME}"
  sudo "${CONTAINER_RUNTIME}" rm "${DB_CONTAINER_NAME}"

  sudo rm -rf "$WORKDIR"

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
  set -eu
}
trap cleanups EXIT

# make a dummy rpm and repo to test payload_repositories
sudo dnf install -y rpm-build createrepo
DUMMYRPMDIR=$(mktemp -d)
DUMMYSPECFILE="$DUMMYRPMDIR/dummy.spec"
export PAYLOAD_REPO_PORT="9999"
export PAYLOAD_REPO_URL="http://localhost:9999"
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
installClient

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

REQUEST_FILE="${WORKDIR}/compose_request.json"
IMG_COMPOSE_REQ_FILE="${WORKDIR}/img_compose_request.json"
ARCH=$(uname -m)
SSH_USER=
TEST_ID="$(uuidgen)"

# Generate a string, which can be used as a predictable resource name,
# especially when running the test in CI where we may need to clean up
# resources in case the test unexpectedly fails or is canceled
CI="${CI:-false}"
CI_BUILD_ID=${CI_BUILD_ID:-$(uuidgen)}
if [[ "$CI" == true ]]; then
  # in CI, imitate GenerateCIArtifactName() from internal/test/helpers.go
  TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_BUILD_ID"
fi
export TEST_ID

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
export SSH_USER

# This removes dot from VERSION_ID.
# ID == rhel   && VERSION_ID == 8.6 => DISTRO == rhel-86
# ID == centos && VERSION_ID == 8   => DISTRO == centos-8
# ID == fedora && VERSION_ID == 35  => DISTRO == fedora-35
export DISTRO="$ID-${VERSION_ID//./}"
SUBSCRIPTION_BLOCK=

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
fi
export SUBSCRIPTION_BLOCK

# Define the customizations for the images here to not have to repeat them
# in every image-type specific file.
case "${IMAGE_TYPE}" in
  # The Directories and Files customization is not supported for this image type.
  "$IMAGE_TYPE_EDGE_INSTALLER")
    DIR_FILES_CUSTOMIZATION_BLOCK=
    ;;
  *)
    DIR_FILES_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
    "directories": [
      {
        "path": "/etc/custom_dir/dir1",
        "user": "root",
        "group": "root",
        "mode": "0775",
        "ensure_parents": true
      },
      {
        "path": "/etc/custom_dir2"
      }
    ],
    "files": [
      {
        "path": "/etc/custom_dir/custom_file.txt",
        "data": "image builder is the best\n"
      },
      {
        "path": "/etc/custom_dir2/empty_file.txt"
      }
    ]
EOF
)
    ;;
esac
export DIR_FILES_CUSTOMIZATION_BLOCK

# generate a temp key for user tests
ssh-keygen -t rsa-sha2-512 -f "${WORKDIR}/usertest" -C "usertest" -N ""

createReqFile

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

    # export for use in subcases
    export UPLOAD_OPTIONS
}

function sendImgFromCompose() {
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
                 https://localhost/api/image-builder-composer/v2/composes/"$COMPOSE_ID"/clone)

    test "$HTTPSTATUS" = "201"
    IMG_ID=$(jq -r '.id' "$OUTPUT")
}

function waitForImgState() {
    while true
    do
        OUTPUT=$(curl \
                     --silent \
                     --show-error \
                     --cacert /etc/osbuild-composer/ca-crt.pem \
                     --key /etc/osbuild-composer/client-key.pem \
                     --cert /etc/osbuild-composer/client-crt.pem \
                     "https://localhost/api/image-builder-composer/v2/clones/$IMG_ID")

        IMG_UPLOAD_STATUS=$(echo "$OUTPUT" | jq -r '.status')
        IMG_UPLOAD_OPTIONS=$(echo "$OUTPUT" | jq -r '.options')

        case "$IMG_UPLOAD_STATUS" in
            "success")
                break
                ;;
            # all valid status values for a compose which hasn't finished yet
            "pending"|"running")
                ;;
            # default undesired state
            "failure")
                echo "Image compose failed"
                exit 1
                ;;
            *)
                echo "API returned unexpected image status value: '$IMG_UPLOAD_STATUS'"
                exit 1
                ;;
        esac

        sleep 30
    done

    # export for use in subcases
    export IMG_UPLOAD_OPTIONS
}

#
# Make sure that requesting a non existing paquet results in failure
#
REQUEST_FILE2="${WORKDIR}/request2.json"
jq '.customizations.packages = [ "jesuisunpaquetquinexistepas" ]' "$REQUEST_FILE" > "$REQUEST_FILE2"

sendCompose "$REQUEST_FILE2"
waitForState "failure"

# crashed/stopped/killed worker should result in the job being retried
sendCompose "$REQUEST_FILE"
waitForState "building"
sudo systemctl stop "osbuild-remote-worker@*"
RETRIED=0
for RETRY in {1..10}; do
    ROWS=$(sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c \
                "SELECT retries FROM jobs WHERE id = '$COMPOSE_ID' AND retries = 1")
    if grep -q "1 row" <<< "$ROWS"; then
        RETRIED=1
        break
    else
        echo "Waiting until job is retried ($RETRY/10)"
        sleep 30
    fi
done
if [ "$RETRIED" != 1 ]; then
    echo "Job $COMPOSE_ID wasn't retried after killing the worker"
    exit 1
fi
# remove the job from the queue so the worker doesn't pick it up again
sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c \
     "DELETE FROM jobs WHERE id = '$COMPOSE_ID'"
sudo systemctl start "osbuild-remote-worker@localhost:8700.service"

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


if [ -s "$IMG_COMPOSE_REQ_FILE" ]; then
    sendImgFromCompose "$IMG_COMPOSE_REQ_FILE"
    waitForImgState
fi

#
# Verify the Cloud-provider specific upload_status options
#
checkUploadStatusOptions

#
# Verify the image landed in the appropriate cloud provider, and delete it.
#
verify

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

until curl --data "grant_type=refresh_token" --output /dev/null --silent --fail localhost:8081/token; do
    sleep 0.5
done

TOKEN="$(curl --request POST \
        --data "grant_type=refresh_token" \
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


sudo systemctl restart osbuild-remote-worker@localhost:8700.service
sudo systemctl is-active --quiet osbuild-remote-worker@localhost:8700.service

exit 0
