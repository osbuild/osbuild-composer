#!/usr/bin/bash

# Handler for bootc guest-image compose with aws.s3 upload target.
# Sources the non-bootc aws.s3 handler to inherit installClient(),
# checkUploadStatusOptions(), and cleanup(). Overrides checkEnv(),
# createReqFile(), and verify() for bootc-specific behavior.

source /usr/libexec/tests/osbuild-composer/api/aws.s3.sh

# Override: check env vars needed for bootc S3 compose
function checkEnv() {
    printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY > /dev/null
    printenv BOOTC_FOUNDRY_DERIVED_CONTAINERS_REGISTRY_USER BOOTC_FOUNDRY_DERIVED_CONTAINERS_REGISTRY_PASS > /dev/null
}

# Override: verify the built image without checking for customizations
# (bootc composes do not include user or package customizations by default)
function verify() {
    local S3_URL
    S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
    greenprint "Verifying S3 object at ${S3_URL}"

    # Tag the resource as a test file
    local S3_FILENAME
    S3_FILENAME=$(echo "import urllib.parse; print(urllib.parse.urlsplit('$S3_URL').path.strip('/'))" | python3 -)

    # tag the object, also verifying that it exists in the bucket as expected
    $AWS_CMD s3api put-object-tagging \
        --bucket "${AWS_BUCKET}" \
        --key "${S3_FILENAME}" \
        --tagging '{"TagSet": [{ "Key": "gitlab-ci-test", "Value": "true" }]}'

    greenprint "✅ Successfully tagged S3 object"

    # Download the disk image and do a basic format check.
    # Bootc images lack a traditional /etc/fstab, so osbuild-image-info
    # cannot parse them.  qemu-img info is enough to confirm the qcow2
    # is structurally valid.
    curl --fail "${S3_URL}" --output "${WORKDIR}/disk.qcow2"

    greenprint "Verifying disk image format with qemu-img"
    qemu-img info "${WORKDIR}/disk.qcow2"

    greenprint "✅ Successfully verified S3 object"
}

# Override: create bootc-specific compose request
function createReqFile() {
    cat > "$REQUEST_FILE" << EOF
{
  "bootc": {
    "reference": "$BOOTC_CONTAINER_REF"
  },
  "image_request": {
    "architecture": "$ARCH",
    "image_type": "${IMAGE_TYPE}",
    "repositories": [],
    "upload_targets": [
      {
        "type": "aws.s3",
        "upload_options": {
          "region": "${AWS_REGION}"
        }
      }
    ]
  }
}
EOF
}
