#!/bin/bash

# Tests the multi-tenancy feature of cloud and remote worker API.
#
# Note that the power of this is very limited. It cannot check that a certain
# tenant can only access jobs on its channel. It has its value though that
# it checks the whole E2E setup including parsing of the JWT token which is
# not tested in the unit test.


set -euo pipefail
set -x

OSBUILD_COMPOSER_TEST_DATA=/usr/share/tests/osbuild-composer/

# Get OS data.
source /usr/libexec/osbuild-composer-test/set-env-variables.sh

# Colorful output.
function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

sudo dnf install -y net-tools

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

greenprint "Registering clean ups"
KILL_PIDS=()
function cleanup() {
  greenprint "== Script execution stopped or finished - Cleaning up =="
  set +eu
  greenprint "Stopping containers"
  sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh stop

  greenprint "Removing generated CA cert"
  sudo rm \
      /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
  sudo update-ca-trust

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
  set -eu
}
trap cleanup EXIT

greenprint "Adding the testsuite's CA cert to the system trust store"
# the worker cannot handle koji with self-signed certs
sudo cp \
    /etc/osbuild-composer/ca-crt.pem \
    /etc/pki/ca-trust/source/anchors/osbuild-composer-tests-ca-crt.pem
sudo update-ca-trust

greenprint "Starting containers"
sudo /usr/libexec/osbuild-composer-test/run-koji-container.sh start

greenprint "Adding kerberos config"
sudo cp \
    /tmp/osbuild-composer-koji-test/client.keytab \
    /etc/osbuild-worker/client.keytab
sudo cp \
    "${OSBUILD_COMPOSER_TEST_DATA}"/kerberos/krb5-local.conf \
    /etc/krb5.conf.d/local

greenprint "Configuring composer and worker"
sudo tee "/etc/osbuild-composer/osbuild-composer.toml" >/dev/null <<EOF
[koji]
enable_tls = false
enable_mtls = false
enable_jwt = true
jwt_keys_urls = ["https://localhost:8082/certs"]
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
jwt_acl_file = ""
jwt_tenant_provider_fields = ["rh-org-id"]
[worker]
enable_artifacts = false
enable_tls = true
enable_mtls = false
enable_jwt = true
jwt_keys_urls = ["https://localhost:8082/certs"]
jwt_ca_file = "/etc/osbuild-composer/ca-crt.pem"
jwt_tenant_provider_fields = ["rh-org-id"]
EOF

# No compose will get this orgID
sudo tee "/etc/osbuild-worker/token" >/dev/null <<EOF
123456789
EOF

sudo tee "/etc/osbuild-worker/osbuild-worker.toml" >/dev/null <<EOF
[authentication]
oauth_url = "http://localhost:8081/token"
client_id = "rhsm-api"
offline_token = "/etc/osbuild-worker/token"

[koji.localhost.kerberos]
principal = "osbuild-krb@LOCAL"
keytab = "/etc/osbuild-worker/client.keytab"

[aws]
bucket = "${AWS_BUCKET}"
credentials = "/etc/osbuild-worker/aws-credentials.toml"
EOF

greenprint "Starting mock servers"
# Spin up an https instance for the composer-api and worker-api; the auth handler needs to hit an ssl `/certs` endpoint
sudo /usr/libexec/osbuild-composer-test/osbuild-mock-openid-provider -a localhost:8082 -rsaPubPem /etc/osbuild-composer/client-crt.pem -rsaPem /etc/osbuild-composer/client-key.pem -cert /etc/osbuild-composer/composer-crt.pem -key /etc/osbuild-composer/composer-key.pem &
KILL_PIDS+=("$!")
# Spin up an http instance for the worker client to bypass the need to specify an extra CA
sudo /usr/libexec/osbuild-composer-test/osbuild-mock-openid-provider -a localhost:8081 -rsaPubPem /etc/osbuild-composer/client-crt.pem -rsaPem /etc/osbuild-composer/client-key.pem &
KILL_PIDS+=("$!")

sudo netstat -putna

greenprint "Restarting composer, stopping a local worker and starting a remote worker"
sudo systemctl restart osbuild-composer
sudo systemctl stop osbuild-worker@1
sudo systemctl start osbuild-remote-worker@localhost:8700

sudo systemctl status 'osbuild*'
sudo netstat -putna

DISTRO=rhel-86

function s3_request {
  cat <<EOF
{
  "distribution": "$DISTRO",
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "guest-image",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "upload_options": {
      "region": "${AWS_REGION}"
    }
  }
}
EOF
}

function koji_request {
  local task_id="$1"
  cat <<EOF
{
  "distribution": "$DISTRO",
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "guest-image",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json)
  },
  "koji": {
    "server": "https://localhost:4343/kojihub",
    "task_id": $task_id,
    "name": "name",
    "version": "version",
    "release": "release"
  }
}
EOF
}

function access_token {
  local refresh_token="$1"
  curl --request POST \
    --data "refresh_token=$refresh_token" \
    --header "Content-Type: application/x-www-form-urlencoded" \
    --silent \
    --show-error \
    --fail \
    localhost:8081/token | jq -r .access_token
}

function send_compose {
  local request_file="$1"
  local refresh_token="$2"
  curl \
    --silent \
    --show-error \
    --fail \
    --header 'Content-Type: application/json' \
    --header "Authorization: Bearer $(access_token "$refresh_token")" \
    --request POST \
    --data @"$request_file" \
    http://localhost:443/api/image-builder-composer/v2/compose | jq -r '.id'
}

function compose_status {
  local compose="$1"
  local refresh_token="$2"
  curl \
    --silent \
    --show-error \
    --fail \
    --header "Authorization: Bearer $(access_token "$refresh_token")" \
    "http://localhost:443/api/image-builder-composer/v2/composes/$compose" | jq -r '.status'
}

function assert_status {
  local compose="$1"
  local refresh_token="$2"
  local status="$3"
  [[ $(compose_status "$compose" "$refresh_token") == "$status" ]]
}

function wait_for_status {
  local compose="$1"
  local refresh_token="$2"
  local desired_status="$3"
  while true
  do
    local current_status
    current_status=$(compose_status "$compose" "$refresh_token")

    case "$current_status" in
      "$desired_status")
        break
        ;;
      # default undesired state
      "failure")
        echo "Image compose failed"
        exit 1
        ;;
    esac

    sleep 10
    done
}

function set_worker_org {
  local org="$1"
  greenprint "Setting worker's org ID to $org"
  sudo tee "/etc/osbuild-worker/token" >/dev/null <<EOF
$org
EOF
  sudo systemctl restart osbuild-remote-worker@localhost:8700
}

ORG=42
greenprint "Sending 1st compose, koji, org id = $ORG"
koji --server=http://localhost:8080/kojihub --user kojiadmin --password kojipass --authtype=password make-task image
ID=$(send_compose <( koji_request 1 ) $ORG)

greenprint "Making sure that a different worker doesn't pick up the compose."
set_worker_org 100
sleep 10
assert_status "$ID" $ORG pending

greenprint "Building the compose."
set_worker_org $ORG
wait_for_status "$ID" $ORG success

ORG=2022
greenprint "Sending 2nd compose, s3, org id = $ORG"
ID=$(send_compose <( s3_request ) $ORG)

greenprint "Making sure that a different worker doesn't pick up the compose."
set_worker_org 42
sleep 10
assert_status "$ID" $ORG pending

greenprint "Building the compose."
set_worker_org $ORG
wait_for_status "$ID" $ORG success
