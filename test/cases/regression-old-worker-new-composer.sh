#!/bin/bash

# Verify that an older worker is still compatible with this composer
# version.
#
# Any tweaks to the worker api need to be backwards compatible.

set -exuo pipefail


ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Only run this on x86
if [ "$ARCH" != "x86_64" ] || [ "$ID" != rhel ] || ! sudo subscription-manager status; then
    echo "Test only supported on RHEL x86_64."
    exit 1
fi

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh

# Check if there is a working repository for the given commit SHA on the S3 AWS bucket.
function check_s3_repo_for_composer_sha {
    if [[ $# -ne 1 ]]; then
        redprint "Usage: check_s3_repo_for_composer_sha <commit-sha>"
        return 1
    fi
    local COMMIT_SHA=$1

    local REPOMD_URL
    REPOMD_URL="http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild-composer/rhel-$(echo "$VERSION_ID" | sed -E 's/\..*//')-cdn/x86_64/${COMMIT_SHA}/repodata/repomd.xml"
    if ! curl --silent --head --fail "$REPOMD_URL" > /dev/null; then
        return 0
    fi
    return 1
}

# Determine the commit SHA of the osbuild-composer worker version we want to test.
# This function will try to find a working repository for the commit SHA corresponding
# to the given version. If that commit doesn't have a working repository, it will try
# to iterate back 10 commits to find a working repository. If not found it will exit
# with error.
#
# The function expects the current worker version as argument.
#
# WARNING - with introduction of a new major version, new `rhel-*-cdn` directory on S3 AWS bucket will be new as well
# that means it will lack the repositories built from commits tagged with previous versions,
# so you need to manually copy them there from directories for minor versions on the same S3 AWS bucket.
function determine_composer_repo_sha {
    if [ $# -ne 1 ]; then
        echo "Usage: determine_composer_repo_sha <composer_version>"
        return 1
    fi
    local COMPOSER_VERSION=$1

    local DESIRED_TAG_SHA
    DESIRED_TAG_SHA=$(curl -s --fail "https://api.github.com/repos/osbuild/osbuild-composer/git/ref/tags/v${COMPOSER_VERSION}" | jq -r '.object.sha')
    if [[ -z "$DESIRED_TAG_SHA" || "$DESIRED_TAG_SHA" == "null" ]]; then
        redprint "Could not find a git tag for osbuild-composer version ${COMPOSER_VERSION}"
        return 1
    fi
    local DESIRED_COMMIT_SHA
    DESIRED_COMMIT_SHA=$(curl -s --fail "https://api.github.com/repos/osbuild/osbuild-composer/git/tags/$DESIRED_TAG_SHA" | jq -r '.object.sha')
    if [[ -z "$DESIRED_COMMIT_SHA" || "$DESIRED_COMMIT_SHA" == "null" ]]; then
        redprint "Could not find a commit SHA for osbuild-composer version ${COMPOSER_VERSION}"
        return 1
    fi

    local TRIES=0
    while [[ $TRIES -lt 10 ]]; do
        if ! check_s3_repo_for_composer_sha "$DESIRED_COMMIT_SHA"; then
            greenprint "Found working composer RPM repository for commit ${DESIRED_COMMIT_SHA}"
            echo "$DESIRED_COMMIT_SHA"
            return
        fi

        yellowprint "No working repository found for commit ${DESIRED_COMMIT_SHA}, trying parent commit"
        DESIRED_COMMIT_SHA=$(curl -s "https://api.github.com/repos/osbuild/osbuild-composer/commits/$DESIRED_COMMIT_SHA" | jq -r '.parents[0].sha')
        TRIES=$((TRIES + 1))
    done
    redprint "Could not find a working repository for any commit in the last 10 versions"
    return 1
}

CURRENT_WORKER_VERSION=$(rpm -q --qf '%{version}\n' osbuild-composer-worker)
# Get the worker version we want to test by comparing the tag for 2 versions
# back - since the current version might still be unreleased, we subtract 3
# from the current version.

OLD_WORKER_VERSION=$((CURRENT_WORKER_VERSION - 3))
DESIRED_WORKER_RPM="osbuild-composer-worker-${OLD_WORKER_VERSION}"
DESIRED_COMMIT_SHA=$(determine_composer_repo_sha "$OLD_WORKER_VERSION")

COMPOSER_CONTAINER_NAME="composer"

# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

greenprint "Copying repository configs from test rpms"
REPOS=$(mktemp -d)
sudo dnf -y install osbuild-composer-tests
sudo cp -a /usr/share/tests/osbuild-composer/repositories "$REPOS/repositories"

greenprint "Stop and disable all services and sockets"
# ignore any errors here
sudo systemctl stop osbuild-composer.service osbuild-composer.socket osbuild-worker@1.service || true
sudo systemctl disable osbuild-composer.service osbuild-composer.socket osbuild-worker@1.service || true

greenprint "Removing latest worker"
sudo dnf remove -y osbuild-composer osbuild-composer-worker osbuild-composer-tests osbuild

function setup_repo {
  local project=$1
  local commit=$2
  local priority=${3:-10}
  local major
  major=$(echo "$VERSION_ID" | sed -E 's/\..*//')

  greenprint "Setting up dnf repository for ${project} ${commit}"
  sudo tee "/etc/yum.repos.d/${project}.repo" << EOF
[${project}]
name=${project} ${commit}
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/${project}/rhel-${major}-cdn/x86_64/${commit}
enabled=1
gpgcheck=0
priority=${priority}
EOF
}

greenprint "Installing osbuild-composer-worker from commit ${DESIRED_COMMIT_SHA}"
setup_repo osbuild-composer "$DESIRED_COMMIT_SHA" 20
sudo dnf install -y osbuild-composer-worker podman composer-cli

# verify the right worker is installed just to be sure
#
# NB: the package version may be lower if the determine_composer_repo_sha
# function had to go back more commits than the number of commits between
# the OLD_WORKER_VERSION version tag and the previous version tag. The
# expectation is that such situations will be rare, but you may need to adjust
# the test if it happens.
rpm -q "$DESIRED_WORKER_RPM"

if type -p podman 2>/dev/null >&2; then
  CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
  CONTAINER_RUNTIME=docker
else
  echo No container runtime found, install podman or docker.
  exit 2
fi


# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

greenprint "Pulling and running composer container for this commit"
sudo "${CONTAINER_RUNTIME}" pull --creds "${V2_QUAY_USERNAME}":"${V2_QUAY_PASSWORD}" \
     "quay.io/osbuild/osbuild-composer-ubi-pr:${CI_COMMIT_SHA}"

cat <<EOF | sudo tee "/etc/osbuild-composer/osbuild-composer.toml"
ignore_missing_repos = true
log_level = "debug"
[koji]
allowed_domains = [ "localhost", "client.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
[worker]
allowed_domains = [ "localhost", "worker.osbuild.org" ]
ca = "/etc/osbuild-composer/ca-crt.pem"
EOF

# The host entitlement doesn't get picked up by composer
# see https://github.com/osbuild/osbuild-composer/issues/1845
sudo "${CONTAINER_RUNTIME}" run  \
     --name=${COMPOSER_CONTAINER_NAME} \
     -d \
     -v /etc/osbuild-composer:/etc/osbuild-composer:Z \
     -v /etc/rhsm:/etc/rhsm:Z \
     -v /etc/pki/entitlement:/etc/pki/entitlement:Z \
     -v "$REPOS/repositories":/usr/share/osbuild-composer/repositories:Z \
     -p 8700:8700 \
     -p 8080:8080 \
     "quay.io/osbuild/osbuild-composer-ubi-pr:${CI_COMMIT_SHA}" \
     --remote-worker-api --no-local-worker-api

greenprint "Wait for composer API"
composer_wait_times=0
while ! openapi=$(curl  --silent  --show-error  --cacert /etc/osbuild-composer/ca-crt.pem  --key /etc/osbuild-composer/client-key.pem  --cert /etc/osbuild-composer/client-crt.pem  https://localhost:8080/api/image-builder-composer/v2/openapi); do
    sleep 10
    composer_wait_times=$((composer_wait_times + 1))

    # wait at maximum 120 seconds for the composer API to come up
    if [[ $composer_wait_times -gt 12 ]]; then
        echo "Composer API did not come up in time"
        sudo "${CONTAINER_RUNTIME}" logs "${COMPOSER_CONTAINER_NAME}"
        exit 1
    fi
done
jq . <<< "${openapi}"


greenprint "Starting osbuild-remote-worker service and dnf-json socket"
set +e
# reload in case there were changes in units
sudo systemctl daemon-reload
sudo systemctl enable --now osbuild-remote-worker@localhost:8700.service
while ! sudo systemctl --quiet is-active osbuild-remote-worker@localhost:8700.service; do
    sudo systemctl status osbuild-remote-worker@localhost:8700.service
    sleep 1
    sudo systemctl enable --now osbuild-remote-worker@localhost:8700.service
done
set -e

# Check that needed variables are set to access AWS.
printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null

# Check that needed variables are set to register to RHSM
printenv API_TEST_SUBSCRIPTION_ORG_ID API_TEST_SUBSCRIPTION_ACTIVATION_KEY_V2 > /dev/null

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

# Set up cleanup functions
# Create a temporary directory and ensure it gets deleted when this script
# terminates in any way.
WORKDIR=$(mktemp -d)
KILL_PIDS=()
function cleanup() {
  greenprint "== Script execution stopped or finished - Cleaning up =="
  set +eu
  cleanupAWSS3

  sudo "${CONTAINER_RUNTIME}" kill composer
  sudo "${CONTAINER_RUNTIME}" rm composer

  sudo rm -rf "$WORKDIR"

  for P in "${KILL_PIDS[@]}"; do
      sudo pkill -P "$P"
  done
  set -eu
}
trap cleanup EXIT

greenprint "Creating dummy rpm and repository to test payload_repositories"
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

greenprint "Installing aws client tools"
if ! hash aws; then
  echo "Using 'awscli' from a container"
  sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_IMAGE_CLOUD_TOOLS}"

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

greenprint "Preparing request"
REQUEST_FILE="${WORKDIR}/request.json"
ARCH=$(uname -m)
CLOUD_PROVIDER="aws.s3"
IMAGE_TYPE="guest-image"

DISTRO="$ID-${VERSION_ID}"

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
    "users": [
      {
        "name": "user1",
        "groups": ["wheel"]
      },
      {
        "name": "user2"
      }
    ]
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\" | .[] | select((has(\"image_type_tags\") | not) or (.\"image_type_tags\" | index(\"${IMAGE_TYPE}\")))" "${REPOS}/repositories/${DISTRO}".json | jq -s .),
    "upload_options": {
      "region": "${AWS_REGION}"
    }
  }
}
EOF

greenprint "Request data"
cat "${REQUEST_FILE}"

#
# Send the request and wait for the job to finish.
#
# Separate `curl` and `jq` commands here, because piping them together hides
# the server's response in case of an error.
#

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
                 https://localhost:8080/api/image-builder-composer/v2/compose)

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
                     "https://localhost:8080/api/image-builder-composer/v2/composes/$COMPOSE_ID")

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

greenprint "Sending compose request to composer"
sendCompose "$REQUEST_FILE"
greenprint "Waiting for success"
waitForState success

test "$UPLOAD_STATUS" = "success"
test "$UPLOAD_TYPE" = "$CLOUD_PROVIDER"

# Verify upload options
S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

# S3 URL contains region and bucket name
echo "$S3_URL" | grep -F "$AWS_BUCKET" -
echo "$S3_URL" | grep -F "$AWS_REGION" -

# verify S3 blob
S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
greenprint "Verifying S3 object at ${S3_URL}"

# Tag the resource as a test file
S3_FILENAME=$(echo "${S3_URL}" | grep -oP '(?<=/)[^/]+(?=\?)')

# tag the object, also verifying that it exists in the bucket as expected
$AWS_CMD s3api put-object-tagging \
    --bucket "${AWS_BUCKET}" \
    --key "${S3_FILENAME}" \
    --tagging '{"TagSet": [{ "Key": "gitlab-ci-test", "Value": "true" }]}'

greenprint "✅ Successfully tagged S3 object"

OSBUILD_GIT_COMMIT=$(cat Schutzfile | jq -r '.["'"${ID}-${VERSION_ID}"'"].dependencies.osbuild.commit')
setup_repo osbuild "$OSBUILD_GIT_COMMIT" 10

greenprint "Installing osbuild-tools for osbuild-image-info"
sudo dnf install -y osbuild-tools

curl "${S3_URL}" --output "${WORKDIR}/disk.qcow2"

# Verify image blobs from s3
function verifyDisk() {
    filename="$1"
    greenprint "Verifying contents of ${filename}"

    infofile="${filename}-info.json"
    sudo osbuild-image-info "${filename}" | tee "${infofile}" > /dev/null

    # save image info to artifacts
    cp -v "${infofile}" "${ARTIFACTS}/image-info.json"

    # extract passwd and packages into separate files
    jq .passwd "${infofile}" > passwd.info
    jq .packages "${infofile}" > packages.info

    # check passwd for blueprint users (user1 and user2)
    if ! grep -q "user1" passwd.info; then
        greenprint "❌ user1 not found in passwd file"
        exit 1
    fi
    if ! grep -q "user2" passwd.info; then
        greenprint "❌ user2 not found in passwd file"
        exit 1
    fi
    # check package list for blueprint packages (postgresql and dummy)
    if ! grep -q "postgresql" packages.info; then
        greenprint "❌ postgresql not found in packages"
        exit 1
    fi
    if ! grep -q "dummy" packages.info; then
        greenprint "❌ dummy not found in packages"
        exit 1
    fi

    greenprint "✅ ${filename} image info verified"
}


verifyDisk "${WORKDIR}/disk.qcow2"
greenprint "✅ Successfully verified S3 object"

greenprint "Test passed!"
exit 0
