#!/usr/bin/bash

#
# This module contains functions for setting up and cleaning up the executor.
#
# It expects the following variables to be set by the caller:
# - CONTAINER_RUNTIME
# - CONTAINER_IMAGE_CLOUD_TOOLS
# - AWS_REGION
# - WORKDIR
# - ID
# - VERSION_ID
# - ARCH
# - GIT_COMMIT or CI_COMMIT_SHA
#
# It exports the following globals:
# - EXECUTOR_KEYPAIR
# - EXECUTOR_KEY_NAME
# - EXECUTOR_SGID
# - EXECUTOR_IP
# - KILL_PIDS
#
# The intended usage is:
# - Call setupExecutorKeypair() to set up the executor keypair
# - Call waitForExecutorInstance() to wait for the executor instance to be ready
# - Call verifyExecutorNetworkIsolation() to verify the executor's network isolation
# - Call provisionExecutor() to provision the executor
# - Call startExecutor() to start the executor
# - Call cleanupExecutor() to clean up the executor
#

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Default SSH user for EC2 instances based on the OS.
# The executor AMI matches the test host, so the cloud-init default user
# follows the same distro convention.
function _executor_ssh_user() {
    case "${ID}" in
        fedora)  echo "fedora" ;;
        *)       echo "ec2-user" ;;
    esac
}

# Full S3 baseurl for a CI RPM repository, matching the path layout used by
# mockbuild.sh (upload) and deploy.sh (install).  On subscribed RHEL (GA
# runners) the RPMs are built against CDN content, so the path contains
# e.g. "rhel-10-cdn" instead of "rhel-10.1".
# Args: $1 = project name (e.g. "osbuild-composer", "osbuild")
#       $2 = commit SHA
function _rpm_repo_baseurl() {
    local project="$1"
    local commit="$2"
    local distro_version="${ID}-${VERSION_ID}"
    if [[ "$ID" == rhel ]] && sudo subscription-manager status &>/dev/null; then
        distro_version="rhel-${VERSION_ID%.*}-cdn"
    fi
    echo "http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/${project}/${distro_version}/${ARCH}/${commit}"
}

# Build an AWS CLI command that authenticates via the EC2 instance role.
# Each function uses this as a local AWS_CMD so it never inherits (or
# overwrites) the caller's AWS_CMD, which may carry explicit IAM credentials
# targeting a different AWS account.
function _executor_aws_cmd() {
    if hash aws 2>/dev/null; then
        echo "aws --region ${AWS_REGION} --output json --color on"
    else
        echo "sudo ${CONTAINER_RUNTIME} run --rm -v ${WORKDIR}:${WORKDIR}:Z ${CONTAINER_IMAGE_CLOUD_TOOLS} aws --region ${AWS_REGION} --output json --color on"
    fi
}

# Sets globals: EXECUTOR_KEYPAIR, INSTANCE_ID, EXECUTOR_KEY_NAME
function setupExecutorKeypair() {
    local AWS_CMD
    AWS_CMD=$(_executor_aws_cmd)

    EXECUTOR_KEYPAIR="${WORKDIR}/executor-keypair.pem"
    INSTANCE_ID=$(curl -Ls http://169.254.169.254/latest/meta-data/instance-id)
    EXECUTOR_KEY_NAME="key-for-${INSTANCE_ID}-executor"

    greenprint "Creating executor keypair ${EXECUTOR_KEY_NAME} (AWS identity follows)"
    $AWS_CMD sts get-caller-identity || true

    $AWS_CMD ec2 create-key-pair \
        --key-name "${EXECUTOR_KEY_NAME}" \
        --query 'KeyMaterial' \
        --output text > "$EXECUTOR_KEYPAIR"
    chmod 400 "$EXECUTOR_KEYPAIR"
    $AWS_CMD ec2 describe-key-pairs --key-names "${EXECUTOR_KEY_NAME}"
}

function cleanupExecutorKeypair() {
    if [ -z "${EXECUTOR_KEY_NAME:-}" ]; then
        return
    fi

    local AWS_CMD
    AWS_CMD=$(_executor_aws_cmd)
    $AWS_CMD ec2 delete-key-pair --key-name "${EXECUTOR_KEY_NAME}"
}

# Sets globals: EXECUTOR_IP, EXECUTOR_SGID
function waitForExecutorInstance() {
    local AWS_CMD
    AWS_CMD=$(_executor_aws_cmd)

    local DESCR_INST="${WORKDIR}/descr-inst.json"
    local RESERVATIONS
    local i

    EXECUTOR_IP=0
    for i in {1..60}; do
        $AWS_CMD ec2 describe-instances \
            --filter "Name=tag:parent,Values=$INSTANCE_ID" > "$DESCR_INST"
        RESERVATIONS=$(jq -r '.Reservations | length' "$DESCR_INST")
        if [ "$RESERVATIONS" -gt 0 ]; then
            EXECUTOR_IP=$(jq -r '.Reservations[0].Instances[0].PrivateIpAddress' "$DESCR_INST")
            break
        fi

        echo "Reservation not ready yet (attempt ${i}/60), waiting..."
        sleep 60
    done

    if [ "$EXECUTOR_IP" = 0 ]; then
        redprint "Unable to find executor host after 60 attempts"
        exit 1
    fi

    EXECUTOR_SGID=$(jq -r '.Reservations[0].Instances[0].SecurityGroups[0].GroupId' "$DESCR_INST")

    local RDY=0
    for i in {0..60}; do
        if ssh-keyscan "$EXECUTOR_IP" > /dev/null 2>&1; then
            RDY=1
            break
        fi
        echo "Waiting for SSH on ${EXECUTOR_IP} (attempt ${i}/60)..."
        sleep 10
    done

    if [ "$RDY" = 0 ]; then
        redprint "Unable to reach executor host $EXECUTOR_IP"
        exit 1
    fi

    greenprint "Executor instance is ready at $EXECUTOR_IP (SG: $EXECUTOR_SGID)"
}

# Verify that the executor's security group only allows egress to the worker host.
# The executor is created with a single egress rule targeting the parent instance.
function verifyExecutorNetworkIsolation() {
    local AWS_CMD
    AWS_CMD=$(_executor_aws_cmd)

    local WORKER_HOST
    WORKER_HOST=$(curl -Ls http://169.254.169.254/latest/meta-data/local-ipv4)

    local DESCR_SGRULE="${WORKDIR}/descr-sgrule.json"
    $AWS_CMD ec2 describe-security-group-rules \
        --filters "Name=group-id,Values=$EXECUTOR_SGID" > "$DESCR_SGRULE"

    local EGRESS_TARGET
    EGRESS_TARGET=$(jq -r '.SecurityGroupRules[] | select(.IsEgress).CidrIpv4' "$DESCR_SGRULE")
    if [ "$EGRESS_TARGET" != "$WORKER_HOST/32" ]; then
        redprint "Executor egress target is $EGRESS_TARGET, expected $WORKER_HOST/32"
        exit 1
    fi

    greenprint "Executor network isolation verified: egress only to $WORKER_HOST/32"
}

function provisionExecutor() {
    local AWS_CMD
    AWS_CMD=$(_executor_aws_cmd)

    local GIT_COMMIT="${GIT_COMMIT:-${CI_COMMIT_SHA}}"
    local OSBUILD_GIT_COMMIT
    OSBUILD_GIT_COMMIT=$(jq -r '.["'"${ID}-${VERSION_ID}"'"].dependencies.osbuild.commit' Schutzfile)

    local COMPOSER_REPO_URL OSBUILD_REPO_URL
    COMPOSER_REPO_URL=$(_rpm_repo_baseurl osbuild-composer "$GIT_COMMIT")
    OSBUILD_REPO_URL=$(_rpm_repo_baseurl osbuild "$OSBUILD_GIT_COMMIT")

    local AUTH_SG="${WORKDIR}/auth-sgrule.json"
    local DESCR_SGRULE="${WORKDIR}/descr-sgrule.json"

    local SSH_USER
    SSH_USER=$(_executor_ssh_user)

    greenprint "Temporarily allowing executor internet access for provisioning"
    $AWS_CMD ec2 authorize-security-group-egress \
        --group-id "$EXECUTOR_SGID" \
        --protocol tcp --cidr 0.0.0.0/0 --port 1-65535 > "$AUTH_SG"
    local SGRULEID
    SGRULEID=$(jq -r '.SecurityGroupRules[0].SecurityGroupRuleId' "$AUTH_SG")

    greenprint "Provisioning executor as ${SSH_USER}@${EXECUTOR_IP}"

    # shellcheck disable=SC2087
    ssh -oStrictHostKeyChecking=no -i "$EXECUTOR_KEYPAIR" "${SSH_USER}@${EXECUTOR_IP}" sudo tee "/etc/yum.repos.d/osbuild.repo" <<EOF
[osbuild-composer]
name=osbuild-composer
baseurl=${COMPOSER_REPO_URL}
enabled=1
gpgcheck=0
priority=10
[osbuild]
name=osbuild
baseurl=${OSBUILD_REPO_URL}
enabled=1
gpgcheck=0
priority=10
EOF

    ssh -oStrictHostKeyChecking=no -i "$EXECUTOR_KEYPAIR" "${SSH_USER}@${EXECUTOR_IP}" \
        sudo dnf install -y osbuild-composer osbuild

    greenprint "Revoking executor internet access"
    $AWS_CMD ec2 revoke-security-group-egress \
        --group-id "$EXECUTOR_SGID" \
        --security-group-rule-ids "$SGRULEID"

    $AWS_CMD ec2 describe-security-group-rules \
        --filters "Name=group-id,Values=$EXECUTOR_SGID" > "$DESCR_SGRULE"

    local SGRULES_LENGTH
    SGRULES_LENGTH=$(jq -r '.SecurityGroupRules | length' "$DESCR_SGRULE")
    if [ "$SGRULES_LENGTH" != 2 ]; then
        redprint "Expected exactly 2 security group rules after revoking internet (got $SGRULES_LENGTH)"
        exit 1
    fi

    greenprint "Verifying executor network isolation after provisioning"
    verifyExecutorNetworkIsolation

    greenprint "Opening worker-executor port on firewall"
    ssh -oStrictHostKeyChecking=no -i "$EXECUTOR_KEYPAIR" "${SSH_USER}@${EXECUTOR_IP}" \
        sudo firewall-cmd --zone=public --add-port=8001/tcp --permanent || true
    ssh -oStrictHostKeyChecking=no -i "$EXECUTOR_KEYPAIR" "${SSH_USER}@${EXECUTOR_IP}" \
        sudo firewall-cmd --reload || true
}

function startExecutor() {
    local SSH_USER
    SSH_USER=$(_executor_ssh_user)

    greenprint "Starting worker executor"
    ssh -oStrictHostKeyChecking=no -i "$EXECUTOR_KEYPAIR" "${SSH_USER}@${EXECUTOR_IP}" \
        sudo /usr/libexec/osbuild-composer/osbuild-worker-executor -host 0.0.0.0 &

    # Re-assign to avoid 'unbound variable' error when array is empty under set -u
    KILL_PIDS=("${KILL_PIDS[@]}")
    KILL_PIDS+=("$!")
}

function cleanupExecutor() {
    cleanupExecutorKeypair
}
