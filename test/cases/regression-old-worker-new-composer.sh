#!/bin/bash

# Verify that an older worker (v33) is still compatible with this composer
# version.
#
# Any tweaks to the worker api need to be backwards compatible.

set -exuo pipefail

WORKER_VERSION=8f21f0b873420a38a261d78a7df130f28b8e2867
WORKER_RPM=osbuild-composer-worker-33-1.20210830git8f21f0b.el8.x86_64

# grab the repos from the test rpms
REPOS=$(mktemp -d)
sudo dnf -y install osbuild-composer-tests
sudo cp -a /usr/share/tests/osbuild-composer/repositories "$REPOS/repositories"
sudo cp -fv "$REPOS/repositories/rhel-8.json" "$REPOS/repositories/rhel-84.json"

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

sudo podman pull --creds "${QUAY_USERNAME}":"${QUAY_PASSWORD}" \
     "quay.io/osbuild/osbuild-composer-ubi-pr:${CI_COMMIT_SHA}"
sudo podman run  \
     --name=composer \
     -d \
     -v /etc/osbuild-composer:/etc/osbuild-composer:Z \
     -v "$REPOS/repositories":/usr/share/osbuild-composer/repositories:Z \
     -v "$WELDR_DIR:/run/weldr/":Z \
     -p 8700:8700 \
     "quay.io/osbuild/osbuild-composer-ubi-pr:${CI_COMMIT_SHA}" \
     --weldr-api --remote-worker-api \
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
if rpm -q --quiet weldr-client; then
    COMPOSE_ID=$(jq -r '.body.build_id' "$COMPOSE_START")
else
    COMPOSE_ID=$(jq -r '.build_id' "$COMPOSE_START")
fi

# Wait for the compose to finish.
echo "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli -s "$WELDR_SOCK" --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    if rpm -q --quiet weldr-client; then
        COMPOSE_STATUS=$(jq -r '.body.queue_status' "$COMPOSE_INFO")
    else
        COMPOSE_STATUS=$(jq -r '.queue_status' "$COMPOSE_INFO")
    fi

    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi

    # Wait 30 seconds and try again.
    sleep 30
done

sudo journalctl -u osbuild-remote-worker@localhost:8700.service
# Verify that the remote worker finished a job
sudo journalctl -u osbuild-remote-worker@localhost:8700.service |
    grep -qE "Job [0-9a-fA-F-]+ finished"

# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi
