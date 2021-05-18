#!/bin/bash
set -xeuo pipefail

ARCH=x86_64

# Provision the software under tet.
/usr/libexec/osbuild-composer-test/provision.sh

# Colorful output.
function greenprint {
    echo -e "\033[1;32m${1}\033[0m"
}

function wait_for_compose {
    COMPOSE_ID=$1
    COMPOSE_INFO=/tmp/compose-info.json

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    set +x
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" > "${COMPOSE_INFO}"
        COMPOSE_STATUS=$(jq -r '.queue_status' "${COMPOSE_INFO}")

        # Is the compose finished?
        if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 5
    done
    set -x
}

sudo dnf install podman -y

sudo systemctl start osbuild-composer.socket
sudo composer-cli status show

greenprint "📋 Preparing blueprint"

EMPTY_BLUEPRINT_FILE=/tmp/empty.toml
EMPTY_BLUEPRINT_NAME=empty
KERNEL_RT_BLUEPRINT_FILE=/tmp/kernel-rt.toml
KERNEL_RT_BLUEPRINT_NAME=kernel-rt

cat > "${EMPTY_BLUEPRINT_FILE}" << STOPHERE
name = "${EMPTY_BLUEPRINT_NAME}"
STOPHERE

cat > "${KERNEL_RT_BLUEPRINT_FILE}" << STOPHERE
name = "${KERNEL_RT_BLUEPRINT_NAME}"

[customizations.kernel]
name = "kernel-rt"
STOPHERE

sudo composer-cli blueprints push "${EMPTY_BLUEPRINT_FILE}"
sudo composer-cli blueprints push "${KERNEL_RT_BLUEPRINT_FILE}"

greenprint "🚀 Starting the first compose"

# Build the first commit, which doesn't have any customizations and serve it over HTTP on localhost:8080

COMPOSE_START=/tmp/compose-start.json
sudo composer-cli --json compose start-ostree --ref "rhel/8/${ARCH}/edge" "${EMPTY_BLUEPRINT_NAME}" rhel-edge-container | tee "${COMPOSE_START}"
COMPOSE_ID=$(jq -r '.build_id' "${COMPOSE_START}")
wait_for_compose "${COMPOSE_ID}"

greenprint "🚀 Importing the container"
PODMAN_LOAD=/tmp/podman-load

mkdir /tmp/images
pushd /tmp/images
sudo composer-cli --json compose image "${COMPOSE_ID}"
cat "${COMPOSE_ID}-rhel84-container.tar" | podman load &> ${PODMAN_LOAD}
PODMNA_IMAGE_ID=$(tail -1 ${PODMAN_LOAD} | cut -d@ -f2)
podman tag "${PODMNA_IMAGE_ID}" localhost/edge-1st
podman run --detach --rm -p 8080:80 --name ostree-repo localhost/edge-1st

greenprint "🚀 Starting the second compose"
# Build the second commit, with kernel-rt and serve it on localhost:8081
#PARENT_COMMIT=$(curl http://127.0.0.1:8080/repo/refs/heads/rhel/8/x86_64/edge)

sudo composer-cli --json compose start-ostree --ref "rhel/8/${ARCH}/edge" --url http://127.0.0.1:8080/repo/ "${KERNEL_RT_BLUEPRINT_NAME}" rhel-edge-container | tee "${COMPOSE_START}"
COMPOSE_ID=$(jq -r '.build_id' "${COMPOSE_START}")
wait_for_compose "${COMPOSE_ID}"
sudo composer-cli --json compose image "${COMPOSE_ID}"
cat "${COMPOSE_ID}-rhel84-container.tar" | podman load &> ${PODMAN_LOAD}
PODMNA_IMAGE_ID=$(tail -1 ${PODMAN_LOAD} | cut -d@ -f2)
podman tag "${PODMNA_IMAGE_ID}" localhost/edge-2nd
podman run --detach --rm -p 8081:80 --name ostree-repo2 localhost/edge-2nd

popd
# Checkout both commits to corresponding filesystem trees and compare the content in /etc/group and /etc/passwd
mkdir /tmp/checkouts
pushd /tmp/checkouts

ostree --repo=repo1 init --mode=archive
ostree --repo=repo1 remote add localhost http://localhost:8080/repo/
echo gpg-verify=false >> repo1/config
ostree --repo=repo1 pull --mirror localhost:rhel/8/x86_64/edge
sudo ostree --repo=repo1 checkout rhel/8/x86_64/edge 1st/

ostree --repo=repo2 init --mode=archive
ostree --repo=repo2 remote add localhost http://localhost:8081/repo/
echo gpg-verify=false >> repo2/config
ostree --repo=repo2 pull --mirror localhost:rhel/8/x86_64/edge
sudo ostree --repo=repo2 checkout rhel/8/x86_64/edge 2nd/

diff 1st/usr/lib/group 2nd/usr/lib/group
diff 1st/usr/lib/passwd 2nd/usr/lib/passwd
