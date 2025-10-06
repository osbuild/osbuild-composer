#!/bin/bash

set -euo pipefail

source /usr/libexec/osbuild-composer-test/set-env-variables.sh
source /usr/libexec/tests/osbuild-composer/shared_lib.sh


# Container image used for cloud provider CLI tools
CONTAINER_IMAGE_CLOUD_TOOLS="quay.io/osbuild/cloud-tools:latest"

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start.json
COMPOSE_INFO=${TEMPDIR}/compose-info.json
DESCR_INST=${TEMPDIR}/descr-inst.json
AUTH_SG=${TEMPDIR}/auth-sgrule.json
DESCR_SGRULE=${TEMPDIR}/descr-sgrule.json
KEYPAIR=${TEMPDIR}/keypair.pem
INSTANCE_ID=$(curl -Ls http://169.254.169.254/latest/meta-data/instance-id)
WORKER_HOST=$(curl -Ls http://169.254.169.254/latest/meta-data/local-ipv4)

# Check available container runtime
if type -p podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif type -p docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME=docker
else
    echo No container runtime found, install podman or docker.
    exit 2
fi

if ! hash aws; then
    echo "Using 'awscli' from a container"
    sudo "${CONTAINER_RUNTIME}" pull ${CONTAINER_IMAGE_CLOUD_TOOLS}

    AWS_CMD="sudo ${CONTAINER_RUNTIME} run --rm \
        -v ${TEMPDIR}:${TEMPDIR}:Z \
        ${CONTAINER_IMAGE_CLOUD_TOOLS} aws --region $AWS_REGION --output json --color on"
else
    echo "Using pre-installed 'aws' from the system"
    AWS_CMD="aws --region $AWS_REGION --output json --color on"
fi
$AWS_CMD --version

subprocessPIDs=()
function cleanup() {
    # since this function can be called at any time, ensure that we don't expand unbound variables
    AWS_CMD="${AWS_CMD:-}"

    if [ -n "$AWS_CMD" ] && [ -f "$KEYPAIR" ]; then
        $AWS_CMD ec2 delete-key-pair --key-name "key-for-$INSTANCE_ID-executor"
    fi

    for p in "${subprocessPIDs[@]}"; do
        sudo pkill -P "$p" || true
    done
}

trap cleanup EXIT

$AWS_CMD ec2 create-key-pair --key-name "key-for-$INSTANCE_ID-executor" --query 'KeyMaterial' --output text > "$KEYPAIR"
chmod 400 "$KEYPAIR"
$AWS_CMD ec2 describe-key-pairs --key-names "key-for-$INSTANCE_ID-executor"

sudo tee "/etc/osbuild-worker/osbuild-worker.toml" <<EOF
[osbuild_executor]
type = "aws.ec2"
key_name = "key-for-$INSTANCE_ID-executor"
EOF

sudo systemctl restart osbuild-worker@1.service

# Write a basic blueprint for our image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "bash"
description = "A base system"
version = "0.0.1"
EOF

sudo composer-cli blueprints push "$BLUEPRINT_FILE"

WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
subprocessPIDs+=( $! )

sudo composer-cli --json compose start bash container | tee "$COMPOSE_START"
COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

EXECUTOR_IP=0
for _ in {1..60}; do
    $AWS_CMD ec2 describe-instances --filter "Name=tag:parent,Values=$INSTANCE_ID" > "$DESCR_INST"
    RESERVATIONS=$(jq -r '.Reservations | length' "$DESCR_INST")
    if [ "$RESERVATIONS" -gt 0 ]; then
        EXECUTOR_IP=$(jq -r .Reservations[0].Instances[0].PrivateIpAddress "$DESCR_INST")
        break
    fi

    echo "Reservation not ready ret, waiting..."
    sleep 60
done

if [ "$EXECUTOR_IP" = 0 ]; then
    redprint "Unable to find executor host"
    exit 1
fi

RDY=0
for _ in {0..60}; do
    if ssh-keyscan "$EXECUTOR_IP" > /dev/null 2>&1; then
        RDY=1
        break
    fi
    sleep 10
done

if [ "$RDY" = 0 ]; then
    redprint "Unable to reach executor host $EXECUTOR_IP"
    exit 1
fi

greenprint "Setting up executor"

# the executor should be created with exactly one egress rule (allowing traffic to the worker host)
SGID=$(jq -r .Reservations[0].Instances[0].SecurityGroups[0].GroupId "$DESCR_INST")
$AWS_CMD ec2 describe-security-group-rules --filters "Name=group-id,Values=$SGID" > "$DESCR_SGRULE"

EGRESS_TARGET=$(jq -r '.SecurityGroupRules[] | select(.IsEgress).CidrIpv4' "$DESCR_SGRULE")
if [ "$EGRESS_TARGET" != "$WORKER_HOST/32" ]; then
    echo executors "$EGRESS_TARGET" is not the expected "$WORKER_HOST/32"
    exit 1
fi

# allow the executor to access the internet for the setup:
$AWS_CMD ec2 authorize-security-group-egress --group-id "$SGID" --protocol tcp --cidr 0.0.0.0/0 --port 1-65535 > "$AUTH_SG"
SGRULEID=$(jq -r .SecurityGroupRules[0].SecurityGroupRuleId "$AUTH_SG")

GIT_COMMIT="${GIT_COMMIT:-${CI_COMMIT_SHA}}"
OSBUILD_GIT_COMMIT=$(cat Schutzfile | jq -r '.["'"${ID}-${VERSION_ID}"'"].dependencies.osbuild.commit')
# shellcheck disable=SC2087
ssh -oStrictHostKeyChecking=no -i "$KEYPAIR" "fedora@$EXECUTOR_IP" sudo tee "/etc/yum.repos.d/osbuild.repo" <<EOF
[osbuild-composer]
name=osbuild-composer
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild-composer/${ID}-${VERSION_ID}/${ARCH}/${GIT_COMMIT}
enabled=1
gpgcheck=0
priority=10
[osbuild]
name=osbuild
baseurl=http://osbuild-composer-repos.s3-website.us-east-2.amazonaws.com/osbuild/${ID}-${VERSION_ID}/${ARCH}/${OSBUILD_GIT_COMMIT}
enabled=1
gpgcheck=0
priority=10
EOF

ssh -oStrictHostKeyChecking=no -i "$KEYPAIR" "fedora@EXECUTOR_IP" sudo journalctl -f &
subprocessPIDs+=( $! )

ssh -oStrictHostKeyChecking=no -i "$KEYPAIR" "fedora@$EXECUTOR_IP" sudo dnf install -y osbuild-composer osbuild

# revoke internet access again during the build
$AWS_CMD ec2 revoke-security-group-egress --group-id "$SGID" --security-group-rule-ids "$SGRULEID"
$AWS_CMD ec2 describe-security-group-rules --filters "Name=group-id,Values=$SGID" > "$DESCR_SGRULE"

SGRULES_LENGTH=$(jq -r '.SecurityGroupRules | length' "$DESCR_SGRULE")
if [ "$SGRULES_LENGTH" != 2 ]; then
    echo "Expected exactly 2 security group rules (got $SGRULES_LENGTH)"
    exit 1
fi

greenprint "🔥 opening worker-executor port on firewall"
ssh -oStrictHostKeyChecking=no -i "$KEYPAIR" "fedora@$EXECUTOR_IP" sudo firewall-cmd --zone=public --add-port=8001/tcp --permanent || true
ssh -oStrictHostKeyChecking=no -i "$KEYPAIR" "fedora@$EXECUTOR_IP" sudo firewall-cmd --reload || true

greenprint "🚀 Starting worker executor"
ssh -oStrictHostKeyChecking=no -i "$KEYPAIR" "fedora@$EXECUTOR_IP" sudo /usr/libexec/osbuild-composer/osbuild-worker-executor -host 0.0.0.0 &
subprocessPIDs+=( $! )

# wait for compose to complete
greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
while true; do
    sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
    COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")
    # Is the compose finished?
    if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
        break
    fi
    sleep 30
done



# Did the compose finish with success?
if [[ $COMPOSE_STATUS != FINISHED ]]; then
    echo "Something went wrong with the compose. 😢"
    exit 1
fi
