#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Global var for ostree ref
OSTREE_REF="test/rhel/8/edge"

function createReqFileEdge() {
  local public_block=

  # on Fedora, upload the artifact publicly, so we can later check the
  # URL created by composer is just public, not presigned
  if [[ $ID == "fedora" ]]; then
    public_block=',"public": true'
  fi

  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "payload_repositories": [
      {
        "baseurl": "$PAYLOAD_REPO_URL"
      }${EXTRA_PAYLOAD_REPOS_BLOCK}
    ],
    "packages": [
      "postgresql",
      "dummy"${EXTRA_PACKAGES_BLOCK}
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
    ]${DIR_FILES_CUSTOMIZATION_BLOCK}
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
    "ostree": {
      "ref": "${OSTREE_REF}"
    },
    "upload_targets": [
      {
        "type": "aws.s3",
        "upload_options": {
          "region": "${AWS_REGION}"${public_block}
        }
      }
    ]
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
      }${EXTRA_PAYLOAD_REPOS_BLOCK}
    ],
    "packages": [
      "postgresql",
      "dummy"${EXTRA_PACKAGES_BLOCK}
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
    ]${SUBSCRIPTION_BLOCK}${DIR_FILES_CUSTOMIZATION_BLOCK}${REPOSITORY_CUSTOMIZATION_BLOCK}${OPENSCAP_CUSTOMIZATION_BLOCK}
${TIMEZONE_CUSTOMIZATION_BLOCK}${FIREWALL_CUSTOMIZATION_BLOCK}${RPM_CUSTOMIZATION_BLOCK}${RHSM_CUSTOMIZATION_BLOCK}${CACERTS_CUSTOMIZATION_BLOCK}
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
      }${EXTRA_PAYLOAD_REPOS_BLOCK}
    ],
    "packages": [
      "postgresql",
      "dummy"${EXTRA_PACKAGES_BLOCK}
    ]${SUBSCRIPTION_BLOCK}${DIR_FILES_CUSTOMIZATION_BLOCK}
${TIMEZONE_CUSTOMIZATION_BLOCK}${FIREWALL_CUSTOMIZATION_BLOCK}${RPM_CUSTOMIZATION_BLOCK}${RHSM_CUSTOMIZATION_BLOCK}
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

# verify edge/iot commit content
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

    verify_dirs_files_customization_edge_commit "${COMMIT_DIR}/repo" $OSTREE_REF
}

function verify_dirs_files_customization_edge_commit() {
  echo "✔️ Checking custom directories and files is ostree commit"
  local _repo_path=$1
  local _ref=$2
  local _error=0

  # verify that `/usr/etc/custom_dir/dir1` exists and has mode `0775`
  # the output from ostree is 'd00775 0 0      0 { [(b'security.selinux', b'system_u:object_r:etc_t:s0')] } /usr/etc/custom_dir/dir1'
  local cust_dir1_mode
  cust_dir1_mode=$(ostree --repo="${_repo_path}" ls -X "${_ref}" /usr/etc/custom_dir/dir1 | awk '{print $1}')
  if [[ "$cust_dir1_mode" != "d00775" ]]; then
    echo "Directory /usr/etc/custom_dir/dir1 has wrong mode: $cust_dir1_mode"
    _error=1
  fi

  # verify that `/usr/etc/custom_dir/custom_file.txt` exists and contains `image builder is the best\n`
  local cust_file_content
  cust_file_content=$(ostree --repo="${_repo_path}" cat "${_ref}" /usr/etc/custom_dir/custom_file.txt)
  if [[ "$cust_file_content" != "image builder is the best" ]]; then
    echo "File /usr/etc/custom_dir/custom_file.txt has wrong content: $cust_file_content"
    _error=1
  fi

  # verify that `/usr/etc/custom_dir2/empty_file.txt` exists and is empty
  local cust_file2_content
  cust_file2_content=$(ostree --repo="${_repo_path}" cat "${_ref}" /usr/etc/custom_dir2/empty_file.txt)
  if [[ "$cust_file2_content" != "" ]]; then
    echo "File /usr/etc/custom_dir2/empty_file.txt has wrong content: $cust_file2_content"
    _error=1
  fi

  if [[ "$_error" == "1" ]]; then
    echo "Testing of custom directories and files failed."
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
