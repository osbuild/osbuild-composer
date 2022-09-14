#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Global var for ostree ref
OSTREE_REF="test/rhel/8/edge"

function createReqFileEdge() {
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
      "region": "${AWS_REGION}"
    }
  }
}
EOF
}

function createReqFileGuest() {
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
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "upload_options": {
      "region": "${AWS_REGION}"
    }
  }
}
EOF
}

# the VSphere test case does not create any additional users,
# since this is not supported by the service UI
function createReqFileVSphere() {
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
      "region": "${AWS_REGION}"
    }
  }
}
EOF
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
