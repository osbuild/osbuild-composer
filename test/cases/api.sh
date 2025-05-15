#!/usr/bin/bash

#
# Test osbuild-composer's main API endpoint by building a sample image and
# uploading it to the appropriate cloud provider. The test currently supports
# AWS and GCP.
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
CLOUD_PROVIDER_OCI="oci"

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
export IMAGE_TYPE_OCI="oci"
export IMAGE_TYPE_VSPHERE="vsphere"
export IMAGE_TYPE_IOT_COMMIT="iot-commit"
export IMAGE_TYPE_IOT_BOOTABLE_CONTAINER="iot-bootable-container"

if (( $# > 2 )); then
    echo "$0 does not support more than two arguments"
    exit 1
fi

if (( $# == 0 )); then
    echo "$0 requires that you set the image type to build"
    exit 1
fi

set -euo pipefail

IMAGE_TYPE="$1"

# set TEST_MODULE_HOTFIXES to 1 to enable module hotfixes for the test
TEST_MODULE_HOTFIXES="${TEST_MODULE_HOTFIXES:-0}"

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
    "$IMAGE_TYPE_OCI")
        CLOUD_PROVIDER="${CLOUD_PROVIDER_OCI}"
        ;;
    "$IMAGE_TYPE_EDGE_COMMIT"|"$IMAGE_TYPE_IOT_COMMIT"|"$IMAGE_TYPE_EDGE_INSTALLER"|"$IMAGE_TYPE_IMAGE_INSTALLER"|"$IMAGE_TYPE_GUEST"|"$IMAGE_TYPE_VSPHERE"|"$IMAGE_TYPE_IOT_BOOTABLE_CONTAINER")
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
if type -p podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
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
    --net host \
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
ignore_missing_repos = true
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
  "$CLOUD_PROVIDER_OCI")
    source /usr/libexec/tests/osbuild-composer/api/oci.sh
    ;;
  "$CLOUD_PROVIDER_CONTAINER_IMAGE_REGISTRY")
    source /usr/libexec/tests/osbuild-composer/api/container.registry.sh
    ;;
  *)
    echo "Unknown cloud provider: ${CLOUD_PROVIDER}"
    exit 1
esac

# Verify that this script is running in the right environment.
greenprint "Verifying environment"
checkEnv
# Check that needed variables are set to register to RHSM (RHEL only)
[[ "$ID" == "rhel" ]] && printenv API_TEST_SUBSCRIPTION_ORG_ID API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2 > /dev/null

function dump_db() {
  # Save the result, including the manifest, for the job, straight from the db
  sudo "${CONTAINER_RUNTIME}" exec "${DB_CONTAINER_NAME}" psql -U postgres -d osbuildcomposer -c "SELECT result FROM jobs WHERE type='manifest-id-only'" \
    | sudo tee "${ARTIFACTS}/build-result.txt" > /dev/null
}

WORKDIR=$(mktemp -d)
KILL_PIDS=()
function cleanups() {
  greenprint "Cleaning up"
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
greenprint "Setting up dummy rpm and repo"
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
greenprint  "Installing cloud provider client tools"
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
if [[ "$CI" == true ]]; then
  # in CI, imitate GenerateCIArtifactName() from internal/test/helpers.go
  TEST_ID="$DISTRO_CODE-$ARCH-$CI_COMMIT_BRANCH-$CI_JOB_ID"
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

export DISTRO="$ID-${VERSION_ID}"
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

# Define the customizations for the images here to not have to repeat them
# in every image-type specific file.
case "${IMAGE_TYPE}" in
  # The Directories and Files customization is not supported for this image type.
  "$IMAGE_TYPE_EDGE_INSTALLER")
    CUSTOM_GPG_KEY=
    REPOSITORY_CUSTOMIZATION_BLOCK=
    ;;
  *)
    CUSTOM_GPG_KEY="-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQGiBGRBSJURBACzCoe9UNfxOUiFLq9b60weSBFdr39mLViscecDWATNvXtgRoK/\nxl/4qpayzALRCQ2Ek/pMrbKPF/3ngECuBv7S+rI4n/rIia4FNcqzYeZAz4DE4NP/\neUGvz49tWhmH17hX/rmF9kz5kLq2bDZI4GDgZW/oMDdt2ivj092Ljm9jRwCgyQy3\nWEK6RJvIcSEh9vbdwVdMPOcD/iHqNejTMFwGyZfCWB0eIOoxUOUn/ZZpELTL2UpW\nGduCf3txb5SkK7M+WDbb0S5IvNXoi0tc13STiD6Oxg2O9PkSvvYb+8zxlhNoSTwy\n54j7Rf5FlnQ3TAFfjtQ5LCx56LKK73j4RjvKW//ktm5n54exsgo9Ry/e12T46dRg\n7tIlA/91rzLm57Qyc73A7zjgIzef9O6V5ZzowC+pp/jfb5pS9hXgROekLkMgX0vg\niA5rM5OpqK4bArVP1lRWnLyvghwO+TW763RVuXlS0scfzMy4g0NgrG6j7TIOKEqz\n4xQxOuwkudqiQr/kOqKuLxQBXa+5MJkyhfPmqYw5wpqyCwFa/7Q4b3NidWlsZCB0\nZXN0IChvc2J1aWxkIHRlc3QgZ3Bna2V5KSA8b3NidWlsZEBleGFtcGxlLmNvbT6I\newQTEQIAOxYhBGB8woiEPRKBO8Cr31lulpQgMejzBQJkQUiVAhsjBQsJCAcCAiIC\nBhUKCQgLAgQWAgMBAh4HAheAAAoJEFlulpQgMejzapMAoLmUg1mNDTRUaCrN/fzm\nHYLHL6jkAJ9pEKkJQiHB6SfD0fkiD2GkELYLubkBDQRkQUiVEAQAlAAXrQ572vuw\nxI3W8GSZmOQiAYOQmOKRloLEy6VZ3NSOb9y2TXj33QTkJBPOM17AzB7E+YjZrpUt\ngl6LlXmfjMcJAcXhFaUBCilAcMwMlLl7DtnSkLnLIXYmHiN0v83BH/H0EPutOc5l\n0QIyugutifp9SJz2+EWpC4bjA7GFkQ8AAwUD/1tLEGqCJ37O8gfzYt2PWkqBEoOY\n0Z3zwVS6PWW/IIkak9dAJ0iX5NMeFWpzFNfviDPHqhEdUR55zsxyUZIZlCX5jwmA\nt7qm3cbH4HNU1Ogq3Q9hykbTPWPZVkpvNm/TO8TA2brhkz3nuS8Hbmh+rjXFOSZj\nDQBUxItuuj2hhpQEiGAEGBECACAWIQRgfMKIhD0SgTvAq99ZbpaUIDHo8wUCZEFI\nlQIbDAAKCRBZbpaUIDHo83fQAKDHgFIaggaNsvDQkj7vMX0fecHRhACfS9Bvxn2W\nWSb6T+gChmYBseZwk/k=\n=DQ3i\n-----END PGP PUBLIC KEY BLOCK-----"
    REPOSITORY_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
    "custom_repositories": [{
        "id": "example",
        "name": "Example repo",
        "baseurl": [ "http://example.com" ],
        "gpgkey": [ "$CUSTOM_GPG_KEY" ],
        "check_gpg": true,
        "enabled": true
    }]
EOF
)
    ;;
esac
export CUSTOM_GPG_KEY
export REPOSITORY_CUSTOMIZATION_BLOCK

# Define the customizations for the images here to not have to repeat them
# in every image-type specific file.
case "${IMAGE_TYPE}" in
  # The Directories and Files customization is not supported for this image type.
  "$IMAGE_TYPE_EDGE_INSTALLER")
    OPENSCAP_CUSTOMIZATION_BLOCK=
    ;;
  *)
    OPENSCAP_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
    "openscap": {
        "profile_id": "pci-dss",
        "policy_id": "1af6cced-581c-452c-89cd-33b7bddb816a",
        "tailoring": {
          "unselected": [ "rpm_verify_permissions" ]
        }
    }
EOF
)
  ;;
esac

export OPENSCAP_CUSTOMIZATION_BLOCK

TIMEZONE_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
    "timezone": {
        "timezone": "Europe/Prague"
    }
EOF
)
export TIMEZONE_CUSTOMIZATION_BLOCK

FIREWALL_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
    "firewall": {
        "services": {
            "enabled": ["nfs"]
        }
    }
EOF
)
export FIREWALL_CUSTOMIZATION_BLOCK

RPM_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
    "rpm": {
      "import_keys": {
        "files": ["/etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-beta"]
      }
    }
EOF
)
# TODO: Remove once the RPM-GPG-KEY-redhat-beta does not use SHA-1
if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} == 10 ]]; then
  yellowprint "RPM-GPG-KEY-redhat-beta uses SHA-1, which is not supported on ${ID}-${VERSION_ID}. No rpm customization applied!"
  RPM_CUSTOMIZATION_BLOCK=
fi
export RPM_CUSTOMIZATION_BLOCK

RHSM_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
    "rhsm": {
      "config": {
        "dnf_plugins": {
          "product_id": {
            "enabled": true
          },
          "subscription_manager": {
            "enabled": false
          }
        },
        "subscription_manager": {
          "rhsm": {
            "manage_repos": true,
            "auto_enable_yum_plugins": false
          },
          "rhsmcertd": {
            "auto_registration": false
          }
        }
      }
    }
EOF
)
export RHSM_CUSTOMIZATION_BLOCK

# Test certificate with common name "Test CA for osbuild", serial 27894af897dd2423607045716438a725f28a6d0b valid until 2298
CACERTS_CUSTOMIZATION_BLOCK=$(cat <<EOF
,
     "cacerts": {
        "pem_certs": [
          "-----BEGIN CERTIFICATE-----\nMIIDszCCApugAwIBAgIUJ4lK+JfdJCNgcEVxZDinJfKKbQswDQYJKoZIhvcNAQEL\nBQAwaDELMAkGA1UEBhMCVVMxFzAVBgNVBAgMDk5vcnRoIENhcm9saW5hMRAwDgYD\nVQQHDAdSYWxlaWdoMRAwDgYDVQQKDAdSZWQgSGF0MRwwGgYDVQQDDBNUZXN0IENB\nIGZvciBvc2J1aWxkMCAXDTI0MDkwMzEzMjkyMFoYDzIyOTgwNjE4MTMyOTIwWjBo\nMQswCQYDVQQGEwJVUzEXMBUGA1UECAwOTm9ydGggQ2Fyb2xpbmExEDAOBgNVBAcM\nB1JhbGVpZ2gxEDAOBgNVBAoMB1JlZCBIYXQxHDAaBgNVBAMME1Rlc3QgQ0EgZm9y\nIG9zYnVpbGQwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAwggEKAoIBAQDeA7OcWTrV\ngstoBsUaeJKm8nelg7Lc0WNXH6yOTLsr4td4yHs0YOvFGwgSf+ffV3RAG1mgqnMG\nMgkD2+z+7QhHbHHs3y0d0zfhA2bg0KVvfCWk7fNRPHY0UOePpXk245Bfw3D0VTpl\nF7nePk1I7ZY09snPWUeb2rjKXzYjKjzM0h27+ykV8I8+FbdyPk/pR8whyDqtHLUa\nXfFy2TFloDSYMkHKVd38BnL0bj91x5F+KsZkN4HzfbYwxLbCQfOSgy7q6TWce9kq\nLo6tya9vuvpWFm1dye7L+BodAQAq/dI/JMeCfyTb0eFb+tyzfr5aVIoqqDN+p9ft\ncw4OefpHbhtNAgMBAAGjUzBRMB0GA1UdDgQWBBRV2A9YmusekPzu5Yf08cV0oPL1\nwjAfBgNVHSMEGDAWgBRV2A9YmusekPzu5Yf08cV0oPL1wjAPBgNVHRMBAf8EBTAD\nAQH/MA0GCSqGSIb3DQEBCwUAA4IBAQCgQZ2Xfj+NxaKBZgn2KNxS0MTbhzHRz6Rn\nqJs+h8OUz2Crmaf6N+RHlmDRZXUrDjSHpxVT2LxFy7ofRrLYIezFDUYfb920VkkV\nSVcxh1YDFROJalfMoE6wdyR/LnK4MJZS9fUpeCJJc/A0J+9FK9CwcyUrHgJ8XbJh\nMKYyQ+cf6O7wzutuBpMyRqSKS+hVM7BQTmSFvv1eAJlo6klGAmmKiYmAEvcQadH1\ndjrujsA3Cn5vX2L+0yuiLB5/zoxqx5cEy97TuKUYB8OqMMujAXNzF4L3HJDUNba2\nAhEkFozMXwYX73TGbGZ0mawPS5D3v3tYTEmJFf6SnVCmUW1fs57g\n-----END CERTIFICATE-----\n"
        ]
      }
EOF
)
export CACERTS_CUSTOMIZATION_BLOCK

if [ "$TEST_MODULE_HOTFIXES" = "1" ]; then
  if [ "$ARCH" = "x86_64" ]; then
    NGINX_REPO_URL="https://rpmrepo.osbuild.org/v2/mirror/public/el8/el8-x86_64-nginx-20240626"
  else
    NGINX_REPO_URL="https://rpmrepo.osbuild.org/v2/mirror/public/el8/el8-aarch64-nginx-20240626"
  fi
  EXTRA_PAYLOAD_REPOS_BLOCK=$(cat <<EOF
,
      {
        "baseurl": "$NGINX_REPO_URL",
        "check_gpg": false,
        "check_repo_gpg": false,
        "rhsm": false,
        "module_hotfixes": true
      }
EOF
)

  EXTRA_PACKAGES_BLOCK=$(cat <<EOF
,
      "nginx",
      "nginx-module-njs"
EOF
)

else
  EXTRA_PAYLOAD_REPOS_BLOCK=""
  EXTRA_PACKAGES_BLOCK=""
fi

export EXTRA_PAYLOAD_REPOS_BLOCK
export EXTRA_PACKAGES_BLOCK

ENABLED_MODULES_BLOCK=
# Only test modularity on rhel 8 and 9
if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} -lt 10 ]]; then
  ENABLED_MODULES_BLOCK=$(cat <<EndOfMessage
,
    "enabled_modules": [
      {
        "name": "nodejs",
        "stream" :"20"
      }
    ]
EndOfMessage
)
fi
export ENABLED_MODULES_BLOCK

# generate a temp key for user tests
ssh-keygen -t rsa-sha2-512 -f "${WORKDIR}/usertest" -C "usertest" -N ""

greenprint  "Creating request file"
createReqFile

#
# Send the request and wait for the job to finish.
#
# Separate `curl` and `jq` commands here, because piping them together hides
# the server's response in case of an error.
#

function collectMetrics(){
    METRICS_OUTPUT=$(curl http://localhost:8008/metrics)

    echo "$METRICS_OUTPUT" | grep "^image_builder_composer_request_count.*path=\"/api/image-builder-composer/v2/compose\"" | cut -f2 -d' '
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

    if [ "$HTTPSTATUS" != "201" ]; then
        redprint "Sending compose request failed:"
        cat "$OUTPUT"
    fi

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

        # get the upload_statuses array to compare the first element with the top level upload_status
        UPLOAD_STATUSES=$(echo "$OUTPUT" | jq -r '.image_status.upload_statuses')

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
# and if TEST_MODULE_HOTFIXES flag is passed, we verify, that without that flag the resolve fails
#

REQUEST_FILE2="${WORKDIR}/request2.json"
jq '.customizations.packages = [ "jesuisunpaquetquinexistepas" ]' "$REQUEST_FILE" > "$REQUEST_FILE2"

greenprint  "Sending compose: Fail test"
sendCompose "$REQUEST_FILE2"
waitForState "failure"

if [ "$TEST_MODULE_HOTFIXES" = "1" ]; then
  cat "$REQUEST_FILE"

  jq 'del(.customizations.payload_repositories[] | select(.baseurl | match(".*public/el8/el8-.*-nginx-.*")) | .module_hotfixes)' "$REQUEST_FILE" > "$REQUEST_FILE2"
  greenprint  "Sending compose: Fail depsolve test"
  sendCompose "$REQUEST_FILE2"
  waitForState "failure"
fi

# crashed/stopped/killed worker should result in the job being retried
greenprint  "Sending compose: Retry test"
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
greenprint  "Sending compose: Full test"
INIT_COMPOSES="$(collectMetrics)"
sendCompose "$REQUEST_FILE"
waitForState
SUBS_COMPOSES="$(collectMetrics)"

test "$UPLOAD_STATUS" = "success"
EXPECTED_UPLOAD_TYPE="$CLOUD_PROVIDER"
if [ "${CLOUD_PROVIDER}" == "${CLOUD_PROVIDER_GENERIC_S3}" ]; then
  EXPECTED_UPLOAD_TYPE="${CLOUD_PROVIDER_AWS_S3}"
fi
if [ "${CLOUD_PROVIDER}" == "${CLOUD_PROVIDER_OCI}" ]; then
    EXPECTED_UPLOAD_TYPE="oci.objectstorage"
fi
test "$UPLOAD_TYPE" = "$EXPECTED_UPLOAD_TYPE"
test $((INIT_COMPOSES+1)) = "$SUBS_COMPOSES"

# test that the first element in the upload_statuses matches the top
# upload_status
UPLOAD_STATUS_0=$(echo "$UPLOAD_STATUSES" | jq -r '.[0].status')
test "$UPLOAD_STATUS_0" = "success"

UPLOAD_TYPE_0=$(echo "$UPLOAD_STATUSES" | jq -r '.[0].type')
test "$UPLOAD_TYPE" = "$UPLOAD_TYPE_0"

UPLOAD_OPTIONS_0=$(echo "$UPLOAD_STATUSES" | jq -r '.[0].options')
test "$UPLOAD_OPTIONS" = "$UPLOAD_OPTIONS_0"


if [ -s "$IMG_COMPOSE_REQ_FILE" ]; then
    sendImgFromCompose "$IMG_COMPOSE_REQ_FILE"
    waitForImgState
fi

#
# Verify the Cloud-provider specific upload_status options
#
greenprint  "Checking upload status options"
checkUploadStatusOptions

#
# Verify the image landed in the appropriate cloud provider, and delete it.
#
greenprint  "Verifying image upload"
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

greenprint  "Verifying package list"
verifyPackageList

#
# Verify oauth2
#
greenprint  "Verifying oauth2"
cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
ignore_missing_repos = true
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

greenprint  "DONE"
exit 0
