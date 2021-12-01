#!/bin/bash

# Verify that an older worker (v33) is still compatible with this composer
# version.
#
# Any tweaks to the worker api need to be backwards compatible.

set -exuo pipefail

function get_build_info() {
    key="$1"
    fname="$2"
    if rpm -q --quiet weldr-client; then
        key=".body${key}"
    fi
    jq -r "${key}" "${fname}"
}

WORKER_VERSION=8f21f0b873420a38a261d78a7df130f28b8e2867
WORKER_RPM=osbuild-composer-worker-33-1.20210830git8f21f0b.el8.x86_64

# grab the repos from the test rpms
REPOS=$(mktemp -d)
sudo dnf -y install osbuild-composer-tests
sudo cp -a /usr/share/tests/osbuild-composer/repositories "$REPOS/repositories"

# Remove the "new" worker
sudo dnf remove -y osbuild-composer osbuild-composer-worker osbuild-composer-tests

function setup_repo {
  local project=$1
  local commit=$2
  local priority=${3:-10}
  echo "Setting up dnf repository for ${project} ${commit}"
  sudo tee "/etc/yum.repos.d/${project}.repo" << EOF
[${project}]
name=${project} ${commit}
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/${project}/rhel-8-cdn/x86_64/${commit}
enabled=1
gpgcheck=0
priority=${priority}
EOF
}

# Composer v33
setup_repo osbuild-composer "$WORKER_VERSION" 20
sudo dnf install -y osbuild-composer-worker podman composer-cli

# verify the right worker is installed just to be sure
rpm -q "$WORKER_RPM"

# run container
WELDR_DIR="$(mktemp -d)"
WELDR_SOCK="$WELDR_DIR/api.socket"

sudo podman pull --creds "${V2_QUAY_USERNAME}":"${V2_QUAY_PASSWORD}" \
     "quay.io/osbuild/osbuild-composer-ubi-pr:${CI_COMMIT_SHA}"

# The host entitlement doesn't get picked up by composer
# see https://github.com/osbuild/osbuild-composer/issues/1845
sudo podman run  \
     --name=composer \
     -d \
     -v /etc/osbuild-composer:/etc/osbuild-composer:Z \
     -v /etc/rhsm:/etc/rhsm:Z \
     -v /etc/pki/entitlement:/etc/pki/entitlement:Z \
     -v "$REPOS/repositories":/usr/share/osbuild-composer/repositories:Z \
     -v "$WELDR_DIR:/run/weldr/":Z \
     -p 8700:8700 \
     "quay.io/osbuild/osbuild-composer-ubi-pr:${CI_COMMIT_SHA}" \
     --weldr-api --dnf-json --remote-worker-api \
     --no-local-worker-api --no-composer-api

# try starting a worker
set +e
sudo systemctl start osbuild-remote-worker@localhost:8700.service
while ! sudo systemctl --quiet is-active osbuild-remote-worker@localhost:8700.service; do
    sudo systemctl status osbuild-remote-worker@localhost:8700.service
    sleep 1
    sudo systemctl start osbuild-remote-worker@localhost:8700.service
done
set -e

function log_on_exit() {
    sudo podman logs composer
}

trap log_on_exit EXIT

BLUEPRINT_FILE=$(mktemp)
COMPOSE_START=$(mktemp)
COMPOSE_INFO=$(mktemp)
tee "$BLUEPRINT_FILE" > /dev/null << EOF2
name = "simple"
version = "0.0.1"

[customizations]
hostname = "simple"
EOF2

sudo composer-cli -s "$WELDR_SOCK" blueprints push "$BLUEPRINT_FILE"
sudo composer-cli -s "$WELDR_SOCK" blueprints depsolve simple
sudo composer-cli -s "$WELDR_SOCK" --json compose start simple qcow2 | tee "${COMPOSE_START}"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli -s "$WELDR_SOCK" --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

sudo composer-cli -s "$WELDR_SOCK" compose delete "${COMPOSE_ID}" >/dev/null

sudo journalctl -u osbuild-remote-worker@localhost:8700.service
# Verify that the remote worker finished a job
sudo journalctl -u osbuild-remote-worker@localhost:8700.service |
    grep -qE "Job [0-9a-fA-F-]+ finished"

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi
