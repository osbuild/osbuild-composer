#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/common.sh

function checkEnv() {
    printenv CI_REGISTRY_USER > /dev/null
    printenv CI_JOB_TOKEN > /dev/null
    printenv CI_REGISTRY > /dev/null
    printenv CI_PROJECT_PATH > /dev/null
}

# Global var for ostree ref
export OSTREE_REF="test/osbuild/edge"
export CONTAINER_SOURCE="registry.gitlab.com/redhat/services/products/image-builder/ci/osbuild-composer/fedora-minimal"

function cleanup() {
    CONTAINER_NAME="${OSTREE_CONTAINER_NAME:-}"
    if [ -n "${CONTAINER_NAME}" ]; then
        sudo "${CONTAINER_RUNTIME}" kill "${CONTAINER_NAME}"
    fi
}

function installClient() {
    local WORKER_CONFIG_DIR="/etc/osbuild-worker"
    local AUTH_FILE_PATH="${WORKER_CONFIG_DIR}/containerauth.json"

    sudo mkdir -p "${WORKER_CONFIG_DIR}"

    # store credentials in authfile and use the file in subsequent calls, see below
    sudo "${CONTAINER_RUNTIME}" login --authfile "${AUTH_FILE_PATH}" --username "${CI_REGISTRY_USER}" --password "${CI_JOB_TOKEN}" "${CI_REGISTRY_IMAGE}"

    cat <<EOF | sudo tee "${WORKER_CONFIG_DIR}/osbuild-worker.toml"
[containers]
auth_file_path="${AUTH_FILE_PATH}"
domain="${CI_REGISTRY}"
path_prefix="${CI_PROJECT_PATH}"
EOF

  sudo systemctl restart "osbuild-remote-worker@localhost:8700"
}

function createReqFile() {
      cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "payload_repositories": [
      {
        "baseurl": "$PAYLOAD_REPO_URL"
      }
    ],
    "containers":[
      {
        "source": "$CONTAINER_SOURCE"
      }
    ],
    "packages": [
      "postgresql",
      "podman",
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
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "ostree": {
      "ref": "${OSTREE_REF}"
    },
    "upload_options": {
      "name": "${DISTRO}-${IMAGE_TYPE}"
    }
  }
}
EOF
}

function checkUploadStatusOptions() {
  local IMAGE_URL_WITHOUT_TAG
  IMAGE_URL_WITHOUT_TAG=$(echo "$UPLOAD_OPTIONS" | jq -r '.url' | awk -F ":" '{print $1}')

  test "${IMAGE_URL_WITHOUT_TAG}" = "${CI_REGISTRY}/${CI_PROJECT_PATH}/${DISTRO}-${IMAGE_TYPE}"
}

function verify() {
  OSTREE_CONTAINER_NAME=osbuild-test
  local IMAGE_URL
  IMAGE_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')

  sudo "${CONTAINER_RUNTIME}" run -d --name osbuild-test -p 8080:8080 "${IMAGE_URL}"

  GET_METADATA_CURL_REQUEST="curl --silent \
      --show-error \
      --cacert /etc/osbuild-composer/ca-crt.pem \
      --key /etc/osbuild-composer/client-key.pem \
      --cert /etc/osbuild-composer/client-crt.pem \
      https://localhost/api/image-builder-composer/v2/composes/${COMPOSE_ID}/metadata"

  BUILD_OSTREE_COMMIT=$(${GET_METADATA_CURL_REQUEST} | jq -r '.ostree_commit')
  SERVICED_OSTREE_COMMIT=$(curl http://localhost:8080/repo/refs/heads/${OSTREE_REF})

  test "${BUILD_OSTREE_COMMIT}" = "${SERVICED_OSTREE_COMMIT}"

  # check the commit contains the container, for this we pull the commit into
  # a repo and poke into the container storage to see if the container we put
  # in there matches the one we expect to be in there.

  # We need to first get the image ID of the container with our architecture
  # and then use its ID to find the config digest
  local CONTAINER_ARCH
  case "$ARCH" in
    "x86_64") CONTAINER_ARCH="amd64" ;;
    "aarch64") CONTAINER_ARCH="arm64" ;;
    "ppc64le") CONTAINER_ARCH="ppc64le" ;;
    "s390x") CONTAINER_ARCH="s390x" ;;
    *) echo "Unknown arch $ARCH"; exit 1 ;;
  esac
  local IMAGE_MANIFEST_ID
  IMAGE_MANIFEST_ID=$(skopeo inspect --raw "docker://$CONTAINER_SOURCE" | jq -r --arg CONTAINER_ARCH "$CONTAINER_ARCH" '.manifests[] | select(.platform.architecture == $CONTAINER_ARCH) | .digest')
  local IMAGE_ID
  IMAGE_ID=$(skopeo inspect --raw "docker://$CONTAINER_SOURCE@$IMAGE_MANIFEST_ID" | jq -r .config.digest)

  ostree init --repo=repo
  ostree remote --repo=repo add --no-gpg-verify container http://localhost:8080/repo
  sudo ostree pull --repo=repo container "${OSTREE_REF}"

  local EMBEDDED_ID
  EMBEDDED_ID=$(sudo ostree cat --repo=repo "${BUILD_OSTREE_COMMIT}" /usr/share/containers/storage/overlay-images/images.json | jq -r .[0].id)

  echo -e "have: sha256:${EMBEDDED_ID}\nwant: ${IMAGE_ID}"
  test "sha256:${EMBEDDED_ID}" = "${IMAGE_ID}"
}
