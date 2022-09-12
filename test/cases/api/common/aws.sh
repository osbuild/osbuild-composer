#!/usr/bin/bash
source /usr/libexec/tests/osbuild-composer/shared_lib.sh

function installClient() {
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

  if ! hash govc; then
    ARCH="$(uname -m)"
    if [ "$ARCH" = "aarch64" ]; then
      ARCH="arm64"
    fi
    greenprint "Installing govc"
    pushd "${WORKDIR}" || exit 1
    curl -Ls --retry 5 --output govc.tar.gz \
         "https://github.com/vmware/govmomi/releases/download/v0.29.0/govc_Linux_$ARCH.tar.gz"
    tar -xvf govc.tar.gz
    GOVC_CMD="${WORKDIR}/govc"
    chmod +x "${GOVC_CMD}"
    popd || exit 1
  else
     echo "Using pre-installed 'govc' from the system"
     GOVC_CMD="govc"
  fi
  $GOVC_CMD version
}

# Log into AWS
# AWS does not need explicit login, but define this function for the sake of
# consistency to allow calling scripts to not care about cloud differences
function cloud_login() {
  true
}
