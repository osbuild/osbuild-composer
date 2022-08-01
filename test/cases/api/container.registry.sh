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

    sudo "${CONTAINER_RUNTIME}" login --authfile "${AUTH_FILE_PATH}" --username "${CI_REGISTRY_USER}" --password "${CI_JOB_TOKEN}" "${CI_REGISTRY_IMAGE}"

    cat <<EOF | sudo tee "${WORKER_CONFIG_DIR}/osbuild-worker.toml"
[containers]
auth_file_path="${AUTH_FILE_PATH}"
domain="${CI_REGISTRY}"
path_prefix="${CI_PROJECT_PATH}"
EOF

  sudo systemctl restart "osbuild-worker@1"
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
}
