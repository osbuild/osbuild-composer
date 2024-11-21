#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/common.sh

# Check that needed variables are set to access GCP.
function checkEnv() {
  printenv GOOGLE_APPLICATION_CREDENTIALS GCP_BUCKET GCP_REGION GCP_API_TEST_SHARE_ACCOUNT > /dev/null
}

function cleanup() {
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


function installClient() {
  if ! hash gcloud; then
    echo "Using 'gcloud' from a container"
    sudo "${CONTAINER_RUNTIME}" pull "${CONTAINER_IMAGE_CLOUD_TOOLS}"

    # directory mounted to the container, in which gcloud stores the credentials after logging in
    GCP_CMD_CREDS_DIR="${WORKDIR}/gcloud_credentials"
    mkdir "${GCP_CMD_CREDS_DIR}"

    GCP_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
      -v ${GCP_CMD_CREDS_DIR}:/root/.config/gcloud:Z \
      -v ${GOOGLE_APPLICATION_CREDENTIALS}:${GOOGLE_APPLICATION_CREDENTIALS}:Z \
      -v ${WORKDIR}:${WORKDIR}:Z \
      ${CONTAINER_IMAGE_CLOUD_TOOLS} gcloud --quiet"
  else
    echo "Using pre-installed 'gcloud' from the system"
    GCP_CMD="gcloud --quiet"
  fi
  $GCP_CMD --version
}


function createReqFile() {
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
    ]${SUBSCRIPTION_BLOCK}${DIR_FILES_CUSTOMIZATION_BLOCK}${REPOSITORY_CUSTOMIZATION_BLOCK}${OPENSCAP_CUSTOMIZATION_BLOCK}
${TIMEZONE_CUSTOMIZATION_BLOCK}${FIREWALL_CUSTOMIZATION_BLOCK}${RPM_CUSTOMIZATION_BLOCK}${RHSM_CUSTOMIZATION_BLOCK}${CACERTS_CUSTOMIZATION_BLOCK}
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "upload_options": {
      "region": "${GCP_REGION}",
      "image_name": "${GCP_IMAGE_NAME}",
      "share_with_accounts": ["${GCP_API_TEST_SHARE_ACCOUNT}"]
    }
  }
}
EOF
}


function checkUploadStatusOptions() {
  GCP_PROJECT=$(jq -r '.project_id' "$GOOGLE_APPLICATION_CREDENTIALS")

  local IMAGE_NAME
  IMAGE_NAME=$(echo "$UPLOAD_OPTIONS" | jq -r '.image_name')
  local PROJECT_ID
  PROJECT_ID=$(echo "$UPLOAD_OPTIONS" | jq -r '.project_id')

  test "$IMAGE_NAME" = "$GCP_IMAGE_NAME"
  test "$PROJECT_ID" = "$GCP_PROJECT"
}

# Log into GCP
function cloud_login() {
  # Authenticate
  $GCP_CMD auth activate-service-account --key-file "$GOOGLE_APPLICATION_CREDENTIALS"
  # Extract and set the default project to be used for commands
  GCP_PROJECT=$(jq -r '.project_id' "$GOOGLE_APPLICATION_CREDENTIALS")
  $GCP_CMD config set project "$GCP_PROJECT"
}

# Verify image in Compute Engine on GCP
function verify() {
  cloud_login

  # Add "gitlab-ci-test" label to the image
  $GCP_CMD compute images add-labels "$GCP_IMAGE_NAME" --labels=gitlab-ci-test=true

  # Verify that the image was shared
  SHARE_OK=1
  $GCP_CMD --format=json compute images get-iam-policy "$GCP_IMAGE_NAME" > "$WORKDIR/image-iam-policy.json"
  SHARED_ACCOUNT=$(jq -r '.bindings[0].members[0]' "$WORKDIR/image-iam-policy.json")
  SHARED_ROLE=$(jq -r '.bindings[0].role' "$WORKDIR/image-iam-policy.json")
  if [ "$SHARED_ACCOUNT" != "$GCP_API_TEST_SHARE_ACCOUNT" ] || [ "$SHARED_ROLE" != "roles/compute.imageUser" ]; then
    SHARE_OK=0
  fi

  if [ "$SHARE_OK" != 1 ]; then
    echo "GCP image wasn't shared with the GCP_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
  fi

  # Verify that the image has guestOsFeatures set
  GCP_IMAGE_GUEST_OS_FEATURES_LEN=$($GCP_CMD compute images describe --project="$GCP_PROJECT" --format="json" "$GCP_IMAGE_NAME" | jq -r '.guestOsFeatures | length')
  if [ "$GCP_IMAGE_GUEST_OS_FEATURES_LEN" -eq 0 ]; then
    echo "❌ Image does not have guestOsFeatures set"
    exit 1
  fi

  # Verify that the image boots and have customizations applied
  # Create SSH keys to use
  GCP_SSH_KEY="$WORKDIR/id_google_compute_engine"
  ssh-keygen -t rsa-sha2-512 -f "$GCP_SSH_KEY" -C "$SSH_USER" -N ""

  # TODO: remove this once el10 / c10s image moves to oslogin
  GCP_METADATA_OPTION=
  # On el10 / c10s, we need to temporarily set the metadata key to "ssh-keys", because there is no "oslogin" feature
  if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} == 10 ]]; then
    GCP_SSH_METADATA_FILE="$WORKDIR/gcp-ssh-keys-metadata"
    echo "${SSH_USER}:$(cat "$GCP_SSH_KEY".pub)" > "$GCP_SSH_METADATA_FILE"
    GCP_METADATA_OPTION="--metadata-from-file=ssh-keys=$GCP_SSH_METADATA_FILE"
  fi

  # create the instance
  # resource ID can have max 62 characters, the $GCP_TEST_ID_HASH contains 56 characters
  GCP_INSTANCE_NAME="vm-$GCP_TEST_ID_HASH"

  # Ensure that we use random GCP region with available 'IN_USE_ADDRESSES' quota
  # We use the CI variable "GCP_REGION" as the base for expression to filter regions.
  # It works best if the "GCP_REGION" is set to a storage multi-region, such as "us"
  local GCP_COMPUTE_REGION
  GCP_COMPUTE_REGION=$($GCP_CMD --format=json compute regions list --filter="name:$GCP_REGION* AND status=UP" | jq -r '.[] | select(.quotas[] as $quota | $quota.metric == "IN_USE_ADDRESSES" and $quota.limit > $quota.usage) | .name' | shuf -n1)

  # Randomize the used GCP zone to prevent hitting "exhausted resources" error on each test re-run
  GCP_ZONE=$($GCP_CMD --format=json compute zones list --filter="region=$GCP_COMPUTE_REGION AND status=UP" | jq -r '.[].name' | shuf -n1)

  # Pick the smallest '^n\d-standard-\d$' machine type from those available in the zone
  local GCP_MACHINE_TYPE
  GCP_MACHINE_TYPE=$($GCP_CMD --format=json compute machine-types list --filter="zone=$GCP_ZONE AND name~^n\d-standard-\d$" | jq -r '.[].name' | sort | head -1)

  # shellcheck disable=SC2086
  $GCP_CMD compute instances create "$GCP_INSTANCE_NAME" \
    --zone="$GCP_ZONE" \
    --image-project="$GCP_PROJECT" \
    --image="$GCP_IMAGE_NAME" \
    --machine-type="$GCP_MACHINE_TYPE" \
    $GCP_METADATA_OPTION --labels=gitlab-ci-test=true

  HOST=$($GCP_CMD --format=json compute instances describe "$GCP_INSTANCE_NAME" --zone="$GCP_ZONE" --format='get(networkInterfaces[0].accessConfigs[0].natIP)')

  echo "⏱ Waiting for GCP instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Verify image
  _ssh="$GCP_CMD compute ssh --strict-host-key-checking=no --ssh-key-file=$GCP_SSH_KEY --zone=$GCP_ZONE $SSH_USER@$GCP_INSTANCE_NAME --"

  # TODO: remove this once el10 / c10s image moves to oslogin
  # On el10 / c10s, we need to ssh directly, because there is no "oslogin" feature
  if [[ ($ID == rhel || $ID == centos) && ${VERSION_ID%.*} == 10 ]]; then
    _ssh="ssh -oStrictHostKeyChecking=no -i $GCP_SSH_KEY $SSH_USER@$HOST"
  fi

  _instanceCheck "$_ssh"
}
