#!/usr/bin/bash

# Handler for bootc bootable-container-iso compose with aws.s3 upload target.
# Sources the guest.s3.sh handler to inherit installClient(),
# checkUploadStatusOptions(), checkEnv(), and cleanup().
# Overrides createReqFile() and verify() for container ISO behavior.

source /usr/libexec/tests/osbuild-composer/api/bootc/guest.s3.sh

# Override: download the ISO from S3 and verify it is a valid ISO image
function verify() {
    local S3_URL
    S3_URL=$(echo "$UPLOAD_OPTIONS" | jq -r '.url')
    greenprint "Verifying S3 object at ${S3_URL}"

    # Tag the resource as a test file
    local S3_FILENAME
    S3_FILENAME=$(echo "import urllib.parse; print(urllib.parse.urlsplit('$S3_URL').path.strip('/'))" | python3 -)

    $AWS_CMD s3api put-object-tagging \
        --bucket "${AWS_BUCKET}" \
        --key "${S3_FILENAME}" \
        --tagging '{"TagSet": [{ "Key": "gitlab-ci-test", "Value": "true" }]}'

    greenprint "Downloading bootable container ISO"
    curl --fail "${S3_URL}" --output "${WORKDIR}/container.iso"

    greenprint "Verifying bootable container ISO image format"
    file "${WORKDIR}/container.iso" | grep -q "ISO 9660"

    greenprint "✅ Successfully verified bootable container ISO"
}

# Override: create bootc bootable-container-iso compose request
function createReqFile() {
    local BOOTC_JSON
    BOOTC_JSON=$(printf '"reference": "%s"' "$BOOTC_CONTAINER_REF")
    if [ -n "$BOOTC_PAYLOAD_REF" ]; then
        BOOTC_JSON=$(printf '%s, "iso_payload_reference": "%s"' "$BOOTC_JSON" "$BOOTC_PAYLOAD_REF")
    fi

    cat > "$REQUEST_FILE" << EOF
{
  "bootc": {
    ${BOOTC_JSON}
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
