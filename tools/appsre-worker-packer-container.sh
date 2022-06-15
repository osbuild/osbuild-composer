#!/bin/bash
set -exv

if [ "$BUILD_RPMS" = true ]; then
    OUTPUT_DIR=/osbuild-composer/

    python3 -m pip install boto3

    mkdir -p "$OUTPUT_DIR/templates/packer/ansible/roles/common/files/rpmbuild/x86_64/RPMS"
    mkdir -p "$OUTPUT_DIR/templates/packer/ansible/roles/common/files/rpmbuild/aarch64/RPMS"

    /osbuild-composer/tools/build-rpms.py --base-dir $OUTPUT_DIR --commit "$COMMIT_SHA" x86_64 aarch64
fi

# Format: PACKER_IMAGE_USERS="\"000000000000\",\"000000000001\""
if [ -n "$PACKER_IMAGE_USERS" ]; then
    cat > /osbuild-composer/templates/packer/share.auto.pkrvars.hcl <<EOF2
image_users = [$PACKER_IMAGE_USERS]
EOF2
fi

/usr/bin/packer build "$PACKER_ONLY_EXCEPT" /osbuild-composer/templates/packer
