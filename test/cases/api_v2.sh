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

source /etc/os-release
DISTRO_CODE="${DISTRO_CODE:-${ID}_${VERSION_ID//./}}"

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

function configure_composer() {
  cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
[koji]
allowed_domains = [ "localhost", "client.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
[koji.aws_config]
bucket = "${AWS_BUCKET}"
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

  sudo mkdir -p /etc/osbuild-worker
  AWS_CREDS_FILE="/etc/osbuild-worker/credentials"
  cat <<EOF | sudo tee "/etc/osbuild-worker/osbuild-worker.toml"
[AWS]
credentials="$AWS_CREDS_FILE"
EOF
  cat <<EOF | sudo tee "$AWS_CREDS_FILE"
[default]
aws_access_key_id = $V2_AWS_ACCESS_KEY_ID
aws_secret_access_key = $V2_AWS_SECRET_ACCESS_KEY
EOF
  sudo systemctl restart osbuild-composer osbuild-composer-worker@*.service
}

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
    configure_composer
    ;;
  "$CLOUD_PROVIDER_GCP")
    exit 0
    ;;
  "$CLOUD_PROVIDER_AZURE")
    exit 0
    ;;
  "$CLOUD_PROVIDER_AWS_S3")
    exit 0
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

# Check that needed variables are set to register to RHSM (RHEL only)
function checkEnvSubscription() {
  printenv API_TEST_SUBSCRIPTION_ORG_ID API_TEST_SUBSCRIPTION_ACTIVATION_KEY > /dev/null
}

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS" | "$CLOUD_PROVIDER_AWS_S3")
    checkEnvAWS
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
  echo "mock cleanup"
}

function cleanupGCP() {
  echo "mock cleanup"
}

function cleanupAzure() {
  echo "mock cleanup"
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


case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS" )
    installClientAWS
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
    https://localhost/api/image-builder-composer/v2/openapi | jq .

#
# Prepare a request to be sent to the composer API.
#

REQUEST_FILE="${WORKDIR}/request.json"
ARCH=$(uname -m)
SSH_USER=

case $(set +x; . /etc/os-release; echo "$ID-$VERSION_ID") in
  "rhel-9.0")
    DISTRO="rhel-90"
    if [[ "$CLOUD_PROVIDER" == "$CLOUD_PROVIDER_AWS" ]]; then
      SSH_USER="ec2-user"
    else
      SSH_USER="cloud-user"
    fi
    ;;
  "rhel-8.6")
    DISTRO="rhel-85"
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
    SSH_USER="cloud-user"
    ;;
  "centos-9")
    DISTRO="centos-9"
    SSH_USER="cloud-user"
    ;;
esac

# Only RHEL need subscription block.
if [[ "$ID" == "rhel" ]]; then
  SUBSCRIPTION_BLOCK=$(cat <<EndOfMessage
,
    "subscription": {
      "organization": "${API_TEST_SUBSCRIPTION_ORG_ID:-}",
      "activation_key": "${API_TEST_SUBSCRIPTION_ACTIVATION_KEY:-}",
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
      "image_type": "aws",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "upload_options": {
        "region": "${AWS_REGION}",
        "share_with_accounts": ["${AWS_API_TEST_SHARE_ACCOUNT}"],
        "snapshot_name": "${AWS_SNAPSHOT_NAME}"
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

    if [[ "$ID" == "rhel" ]]; then
        echo "rhel --------------------------------- "
        cat "$REQUEST_FILE"
        echo "rhel --------------------------------- "
    fi
    OUTPUT=$(curl \
                 --silent \
                 --show-error \
                 --cacert /etc/osbuild-composer/ca-crt.pem \
                 --key /etc/osbuild-composer/client-key.pem \
                 --cert /etc/osbuild-composer/client-crt.pem \
                 --header 'Content-Type: application/json' \
                 --request POST \
                 --data @"$REQUEST_FILE" \
                 https://localhost/api/image-builder-composer/v2/compose)

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
                     https://localhost/api/image-builder-composer/v2/composes/"$COMPOSE_ID")

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
SUBS_COMPOSES="$(collectMetrics)"

test "$UPLOAD_STATUS" = "success"
test "$UPLOAD_TYPE" = "$CLOUD_PROVIDER"
test $((INIT_COMPOSES+1)) = "$SUBS_COMPOSES"


# Make sure we get 1 job entry in the db per compose
sudo podman exec osbuild-composer-db psql -U postgres -d osbuildcomposer -c "SELECT COUNT(*) FROM jobs;"

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

case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    checkUploadStatusOptionsAWS
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


case $CLOUD_PROVIDER in
  "$CLOUD_PROVIDER_AWS")
    verifyInAWS
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
    https://localhost/api/image-builder-composer/v2/compose)" = "400" ]

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
    https://localhost/api/image-builder-composer/v2/compose)" = "500" ]

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
allowed_domains = [ "localhost", "worker.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
pg_host = "localhost"
pg_port = "5432"
pg_database = "osbuildcomposer"
pg_user = "postgres"
pg_password = "foobar"
pg_ssl_mode = "disable"
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
        http://localhost:443/api/image-builder-composer/v2/composes/"$COMPOSE_ID")" = "200" ]

[ "$(curl \
        --silent \
        --output /dev/null \
        --write-out '%{http_code}' \
        --header "Authorization: Bearer badtoken" \
        http://localhost:443/api/image-builder-composer/v2/composes/"$COMPOSE_ID")" = "401" ]

sudo systemctl start osbuild-remote-worker@https:--localhost:8700.service
sudo systemctl is-active --quiet osbuild-remote-worker@https:--localhost:8700.service

exit 0
