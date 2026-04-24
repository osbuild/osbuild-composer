#!/usr/bin/bash

#
# Supported image type name constants.
#
# Sourced by test entry points (api.sh, api-bootc-service.sh, …) so that
# handler scripts in api/ and api/bootc/ can reference them freely.
#

export IMAGE_TYPE_AWS="aws"
export IMAGE_TYPE_AZURE="azure"
export IMAGE_TYPE_BOOTABLE_CONTAINER_ISO="bootable-container-iso"
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
