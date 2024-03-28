#!/bin/bash
set -euo pipefail

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)
source /usr/libexec/tests/osbuild-composer/ostree-common-functions.sh

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

common_init

# Set up variables.
TEST_UUID=$(uuidgen)
IMAGE_KEY="edge-${TEST_UUID}"
PROD_REPO_URL=http://192.168.100.1/repo
PROD_REPO=/var/www/html/repo
STAGE_REPO_ADDRESS=192.168.200.1
STAGE_REPO_URL="http://${STAGE_REPO_ADDRESS}:8080/repo/"
ARTIFACTS="${ARTIFACTS:-/tmp/artifacts}"
CONTAINER_TYPE=edge-container
CONTAINER_FILENAME=container.tar
AMI_IMAGE_TYPE=edge-ami
AMI_IMAGE_FILENAME=image.raw
OSTREE_OSNAME=redhat
BUCKET_NAME="composer-ci-${TEST_UUID}"
BUCKET_URL="s3://${BUCKET_NAME}"
OBJECT_URL="http://${BUCKET_NAME}.s3.${AWS_DEFAULT_REGION}.amazonaws.com"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
export COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
export COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)
IGNITION_USER=core
IGNITION_USER_PASSWORD="${IGNITION_USER_PASSWORD:-foobar}"
IGNITION_USER_PASSWORD_SHA512=$(openssl passwd -6 -stdin <<< "${IGNITION_USER_PASSWORD}")

# Set FIPS variable default
FIPS="${FIPS:-false}"

# Generate the user's password hash
EDGE_USER_PASSWORD="${EDGE_USER_PASSWORD:-foobar}"
EDGE_USER_PASSWORD_SHA512=$(openssl passwd -6 -stdin <<< "${EDGE_USER_PASSWORD}")

case "${ID}-${VERSION_ID}" in
    "rhel-9."*)
        OSTREE_REF="rhel/9/${ARCH}/edge"
        SYSROOT_RO="true"
        ;;
    "centos-9")
        OSTREE_REF="centos/9/${ARCH}/edge"
        SYSROOT_RO="true"
        ;;
    *)
        redprint "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac

# Test result checking
check_result () {
    greenprint "🎏 Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        redprint "❌ Failed"
        clean_up
        aws_clean_up
        exit 1
    fi
}

# Configure AWS EC2 network
add_vpc () {
    # Network setup
    greenprint "VPC Network setup."

    # Create VPC
    VPC_ID=$(
        aws ec2 create-vpc \
            --output json \
            --tag-specification 'ResourceType=vpc,Tags=[{Key=Name,Value=kite-ci}]' \
            --cidr-block 172.32.0.0/16 \
            --region="${AWS_DEFAULT_REGION}" | jq -r '.Vpc.VpcId'
    )

    # Create VPC Internet Gateway
    IGW_ID=$(
        aws ec2 create-internet-gateway \
            --output json \
            --tag-specifications 'ResourceType=internet-gateway,Tags=[{Key=Name,Value=kite-ci}]' | \
            jq -r '.InternetGateway.InternetGatewayId'
    )

    # Attach internet gateway
    aws ec2 attach-internet-gateway \
        --vpc-id "${VPC_ID}" \
        --internet-gateway-id "${IGW_ID}"

    # Add default route in route table for all vpc subnets
    # Create route table
    RT_ID=$(
        aws ec2 create-route-table \
            --output json \
            --vpc-id "${VPC_ID}" \
            --tag-specifications 'ResourceType=route-table,Tags=[{Key=Name,Value=kite-ci}]' | \
            jq -r '.RouteTable.RouteTableId'
    )

    aws ec2 create-route \
        --route-table-id "${RT_ID}" \
        --destination-cidr-block 0.0.0.0/0 \
        --gateway-id "${IGW_ID}"

    ALL_ZONES=( "us-east-1a" "us-east-1b" "us-east-1c" "us-east-1d" "us-east-1e" "us-east-1f" )
    LENGTH=${#ALL_ZONES[@]}
    for (( j=0; j<LENGTH; j++ ))
    do
        # Create Subnet for VPC
        SUBN_ID=$(
            aws ec2 create-subnet \
                --output json \
                --vpc-id "${VPC_ID}" \
                --cidr-block "172.32.3${j}.0/24" \
                --availability-zone "${ALL_ZONES[$j]}" \
                --tag-specifications 'ResourceType=subnet,Tags=[{Key=Name,Value=kite-ci}]' | \
                jq -r '.Subnet.SubnetId'
        )
        # Associate route table to subnet
        aws ec2 associate-route-table \
            --route-table-id "${RT_ID}" \
            --subnet-id "${SUBN_ID}"
    done

    # Security Group setup
    SEC_GROUP_ID=$(
        aws ec2 create-security-group \
            --output json \
            --group-name kite-ci-sg \
            --description "kite ci edge-ami security group" \
            --vpc-id "${VPC_ID}" \
            --tag-specifications 'ResourceType=security-group,Tags=[{Key=Name,Value=kite-ci}]' | \
            jq -r '.GroupId'
    )
    # Allow inbound ssh connections
    aws ec2 authorize-security-group-ingress \
        --group-id "${SEC_GROUP_ID}" \
        --protocol tcp \
        --port 22 \
        --cidr 0.0.0.0/0 \
        --tag-specifications 'ResourceType=security-group-rule,Tags=[{Key=Name,Value=kite-ci}]'
}

# Get instance type
get_instance_type () {
    arch=$1

    if [[ $arch == x86_64 ]]; then
        allInstanceTypes=( \
            "t2.medium" \
            "t3.medium" \
            "m6a.large" \
        )
    elif [[ $arch == aarch64 ]]; then
        allInstanceTypes=( \
            "t4g.medium" \
            "c7g.medium" \
            "m6g.medium" \
        )
    else
        echo "Not supported Architecture"
        exit 1
    fi
    RND_LINE=$((RANDOM % 3))
    echo "${allInstanceTypes[$RND_LINE]}"
}

###########################################################
##
## Prepare edge prod and stage repo
##
###########################################################
greenprint "🔧 Prepare edge prod repo for ami test"
# Start prod repo web service
# osbuild-composer-tests have mod_ssl as a dependency. The package installs
# an example configuration which automatically enabled httpd on port 443, but
# that one is already in use. Remove the default configuration as it is useless
# anyway.
sudo rm -f /etc/httpd/conf.d/ssl.conf
sudo systemctl enable --now httpd.service

# Have a clean prod repo
sudo rm -rf "$PROD_REPO"
sudo mkdir -p "$PROD_REPO"
sudo ostree --repo="$PROD_REPO" init --mode=archive
sudo ostree --repo="$PROD_REPO" remote add --no-gpg-verify edge-stage "$STAGE_REPO_URL"

# Prepare stage repo network
greenprint "🔧 Prepare stage repo network"
sudo podman network inspect edge >/dev/null 2>&1 || sudo podman network create --driver=bridge --subnet=192.168.200.0/24 --gateway=192.168.200.254 edge

##########################################################
##
## Build edge-container image and start it in podman
##
##########################################################

# Write a blueprint for ostree image.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "container"
description = "A base rhel-edge container image"
version = "0.0.1"
modules = []
groups = []

[[packages]]
name = "python3"
version = "*"
EOF

# Red Hat does not provide realtime kernel package for ARM
if [[ "${ARCH}" != aarch64 ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[customizations.kernel]
name = "kernel-rt"
EOF
fi

greenprint "📄 container blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing container blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve container

# Build container image.
build_image -b container -t "${CONTAINER_TYPE}"

# Download the image
greenprint "📥 Downloading the container image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

# Clear stage repo running env
greenprint "🧹 Clearing stage repo running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

# Deal with stage repo image
greenprint "🗜 Starting container"
IMAGE_FILENAME="${COMPOSE_ID}-${CONTAINER_FILENAME}"
sudo podman pull "oci-archive:${IMAGE_FILENAME}"
sudo podman images
# Run edge stage repo
greenprint "🛰 Running edge stage repo"
# Get image id to run image
EDGE_IMAGE_ID=$(sudo podman images --filter "dangling=true" --format "{{.ID}}")
sudo podman run -d --name rhel-edge --network edge --ip "$STAGE_REPO_ADDRESS" "$EDGE_IMAGE_ID"
# Clear image file
sudo rm -f "$IMAGE_FILENAME"

# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Sync edge content
greenprint "📡 Sync content from stage repo"
sudo ostree --repo="$PROD_REPO" pull --mirror edge-stage "$OSTREE_REF"

# Clean compose and blueprints.
greenprint "🧽 Clean up container blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete container > /dev/null

############################################################
##
## Setup Ignition
##
############################################################

IGNITION_CONFIG_PATH="./config.ign"
sudo tee "$IGNITION_CONFIG_PATH" > /dev/null << EOF
{
  "ignition": {
    "config": {
      "merge": [
        {
          "source": "${OBJECT_URL}/sample.ign"
        }
      ]
    },
    "timeouts": {
      "httpTotal": 30
    },
    "version": "3.3.0"
  },
  "passwd": {
    "users": [
      {
        "groups": [
          "wheel"
        ],
        "name": "$IGNITION_USER",
        "passwordHash": "${IGNITION_USER_PASSWORD_SHA512}",
        "sshAuthorizedKeys": [
          "$SSH_KEY_PUB"
        ]
      }
    ]
  }
}
EOF

IGNITION_CONFIG_SAMPLE_PATH="./sample.ign"
sudo tee "$IGNITION_CONFIG_SAMPLE_PATH" > /dev/null << EOF
{
  "ignition": {
    "version": "3.3.0"
  },
  "storage": {
    "files": [
      {
        "path": "/usr/local/bin/startup.sh",
        "contents": {
          "compression": "",
          "source": "data:;base64,IyEvYmluL2Jhc2gKZWNobyAiSGVsbG8sIFdvcmxkISIK"
        },
        "mode": 493
      }
    ]
  },
  "systemd": {
    "units": [
      {
        "contents": "[Unit]\nDescription=A hello world unit!\n[Service]\nType=oneshot\nRemainAfterExit=yes\nExecStart=/usr/local/bin/startup.sh\n[Install]\nWantedBy=multi-user.target\n",
        "enabled": true,
        "name": "hello.service"
      },
      {
        "dropins": [
          {
            "contents": "[Service]\nEnvironment=LOG_LEVEL=trace\n",
            "name": "log_trace.conf"
          }
        ],
        "name": "fdo-client-linuxapp.service"
      }
    ]
  }
}
EOF
sudo chmod +r "${IGNITION_CONFIG_SAMPLE_PATH}" "${IGNITION_CONFIG_PATH}"

# Start AWS cli installation
curl "https://awscli.amazonaws.com/awscli-exe-linux-${ARCH}.zip" -o "awscliv2.zip"
unzip awscliv2.zip > /dev/null
sudo ./aws/install --update
aws --version

# Clean up unzipped folder and files
sudo rm -rf awscliv2.zip ./aws

# Create Bucket
aws s3 mb \
    "${BUCKET_URL}" \
    --region "${AWS_DEFAULT_REGION}"
# Disable Public Access Block
aws s3api put-public-access-block \
    --bucket "${BUCKET_NAME}" \
    --public-access-block-configuration "BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false"
# Set Object ownership
aws s3api put-bucket-ownership-controls \
    --bucket "${BUCKET_NAME}" \
    --ownership-controls="Rules=[{ObjectOwnership=BucketOwnerPreferred}]"

# Upload ignition files to bucket
greenprint "📂 Upload ignition files to AWS S3 bucket"
aws s3 cp \
    "${IGNITION_CONFIG_PATH}" \
    "${BUCKET_URL}/" \
    --acl public-read
aws s3 cp \
    "${IGNITION_CONFIG_SAMPLE_PATH}" \
    "${BUCKET_URL}/" \
    --acl public-read
sudo rm -rf "${IGNITION_CONFIG_PATH}" "${IGNITION_CONFIG_SAMPLE_PATH}"

############################################################
##
## Build edge-ami
##
############################################################

# Write a blueprint for raw ami.
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "ami"
description = "A rhel-edge ami"
version = "0.0.1"
modules = []
groups = []
EOF

if [ "${FIPS}" == "true" ]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[customizations]
fips = ${FIPS}
EOF
fi

tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "${EDGE_USER_PASSWORD_SHA512}"
key = "${SSH_KEY_PUB}"
home = "/home/admin/"
groups = ["wheel"]

[customizations.ignition.firstboot]
url = "${OBJECT_URL}/config.ign"
EOF

greenprint "📄 aws ami blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing edge ami blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve ami

# Build ami.
build_image -b ami -t "${AMI_IMAGE_TYPE}" -u "${PROD_REPO_URL}"

# Download the image
greenprint "📥 Downloading the ami image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
AMI_FILENAME="${COMPOSE_ID}-${AMI_IMAGE_FILENAME}"
# Configure ami file with correct permissions
sudo chmod +r "${AMI_FILENAME}"

# Upload ami to AWS S3 bucket
greenprint "📂 Upload raw ami to S3 bucket"
aws s3 cp \
    --quiet \
    "${AMI_FILENAME}" \
    "${BUCKET_URL}/" \
    --acl public-read
sudo rm -f "$AMI_FILENAME"

# Clean compose and blueprints
greenprint "🧹 Clean up edge-ami compose and blueprint"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete ami > /dev/null

# Create container simple file
CONTAINERS_FILE=containers.json

tee "$CONTAINERS_FILE" > /dev/null << EOF
{
  "Description": "${AMI_FILENAME}",
  "Format": "raw",
  "Url": "${BUCKET_URL}/${AMI_FILENAME}"
}
EOF

# Import the image as an EBS snapshot into EC2
IMPORT_TASK_ID=$(
    aws ec2 import-snapshot \
        --output json \
        --description "RHEL edge ami snapshot" \
        --disk-container file://"${CONTAINERS_FILE}" | \
        jq -r '.ImportTaskId'
)
rm -f "$CONTAINERS_FILE"

# Wait for snapshot import complete
for _ in $(seq 0 180); do
    IMPORT_STATUS=$(
        aws ec2 describe-import-snapshot-tasks \
            --output json \
            --import-task-ids "${IMPORT_TASK_ID}" | \
            jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.Status'
    )

    # Has the snapshot finished?
    if [[ $IMPORT_STATUS != active ]]; then
        break
    fi

    # Wait 10 seconds and try again.
    sleep 10
done

if [[ $IMPORT_STATUS != completed ]]; then
    echo "Something went wrong with the snapshot. 😢"
    exit 1
else
    greenprint "Snapshot imported successfully."
fi

SNAPSHOT_ID=$(
    aws ec2 describe-import-snapshot-tasks \
        --output json \
        --import-task-ids "${IMPORT_TASK_ID}" | \
        jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.SnapshotId'
)

aws ec2 create-tags \
    --resources "${SNAPSHOT_ID}" \
    --tags Key=Name,Value=composer-ci Key=UUID,Value="$TEST_UUID"

# Import  keypair
greenprint "Share ssh public key with AWS"
AMI_KEY_NAME="edge-ami-key-${TEST_UUID}"
# Clean previous configured keypair
aws ec2 import-key-pair \
    --key-name "${AMI_KEY_NAME}" \
    --public-key-material fileb://"${SSH_KEY}".pub \
    --tag-specification 'ResourceType=key-pair,Tags=[{Key=Name,Value=composer-ci}]'

# Create ec2 network
EXISTED_VPC=$(
    aws ec2 describe-vpcs \
        --filters="Name=tag:Name,Values=kite-ci" \
        --output json \
        --query "Vpcs"
)
if [[ "$EXISTED_VPC" == "[]" ]]; then
    add_vpc
fi

##################################################################
##
## Install and test edge EC2 instance with edge-ami image
##
##################################################################
# Create AMI image from EBS snapshot
greenprint "Register AMI, create image from snapshot."
REGISTERED_AMI_NAME="edge_ami-${TEST_UUID}"

if [[ "${ARCH}" == x86_64 ]]; then
    IMG_ARCH="${ARCH}"
elif [[ "${ARCH}" == aarch64 ]]; then
    IMG_ARCH=arm64
fi

AMI_ID=$(
    aws ec2 register-image \
        --name "${REGISTERED_AMI_NAME}" \
        --root-device-name /dev/xvda \
        --architecture "${IMG_ARCH}" \
        --ena-support \
        --sriov-net-support simple \
        --virtualization-type hvm \
        --block-device-mappings DeviceName=/dev/xvda,Ebs=\{SnapshotId="${SNAPSHOT_ID}"\} DeviceName=/dev/xvdf,Ebs=\{VolumeSize=10\} \
        --boot-mode uefi-preferred \
        --output json | \
        jq -r '.ImageId'
)

# Wait for image available to use to avoid image not available error
aws ec2 wait image-available \
    --image-ids "$AMI_ID"

aws ec2 create-tags \
    --resources "${AMI_ID}" \
    --tags Key=Name,Value=composer-ci Key=UUID,Value="$TEST_UUID"

# Create instance market options
MARKET_OPTIONS=spot-options.json
tee "${MARKET_OPTIONS}" > /dev/null << EOF
{
  "MarketType": "spot",
  "SpotOptions": {
    "MaxPrice": "0.1",
    "SpotInstanceType": "one-time",
    "InstanceInterruptionBehavior": "terminate"
  }
}
EOF

# Launch Instance
greenprint "💻 Launch instance from AMI"
for _ in $(seq 0 9); do
    RESULTS=0
    INSTANCE_OUT_INFO=instance_output_info.json
    INSTANCE_TYPE=$(get_instance_type "${ARCH}")

    ZONE_LIST=$(
        aws ec2 describe-instance-type-offerings \
            --location-type availability-zone \
            --filters="Name=instance-type,Values=${INSTANCE_TYPE}" \
            --query "InstanceTypeOfferings"
    )
    if [[ "$ZONE_LIST" == "[]" ]]; then
        greenprint "No available $INSTANCE_TYPE in this region"
        break
    else
        ZONE_NAME=$(echo "$ZONE_LIST" | jq -r ".[0].Location")
    fi
    SUBNET_ID=$(
        aws ec2 describe-subnets \
            --output json \
            --filters "Name=tag:Name,Values=kite-ci" "Name=availabilityZone,Values=${ZONE_NAME}" | \
            jq -r ".Subnets[0].SubnetId"
    )
    SEC_GROUP_ID=$(
        aws ec2 describe-security-groups \
            --filters="Name=tag:Name,Values=kite-ci" \
            --output json | \
            jq -r ".SecurityGroups[0].GroupId"
    )

    aws ec2 run-instances \
        --image-id "${AMI_ID}" \
        --count 1 \
        --instance-type "${INSTANCE_TYPE}" \
        --placement AvailabilityZone="${ZONE_NAME}" \
        --tag-specification "ResourceType=instance,Tags=[{Key=Name,Value=composer-ci},{Key=UUID,Value=$TEST_UUID}]" \
        --instance-market-options file://"${MARKET_OPTIONS}" \
        --key-name "${AMI_KEY_NAME}" \
        --security-group-ids "${SEC_GROUP_ID}" \
        --subnet-id "${SUBNET_ID}" \
        --associate-public-ip-address > "${INSTANCE_OUT_INFO}" 2>&1 || :
    if ! grep -iqE 'unsupported|InsufficientInstanceCapacity' "${INSTANCE_OUT_INFO}"; then
        echo "Instance type supported!"
        RESULTS=1
        break
    fi
    sleep 30
done
cat "${INSTANCE_OUT_INFO}"

# Check instance has been deployed correctly
check_result

INSTANCE_ID=$(jq -r '.Instances[].InstanceId' "${INSTANCE_OUT_INFO}")

# wait for instance running
aws ec2 wait instance-running \
    --instance-ids "$INSTANCE_ID"

# get instance public ip
PUBLIC_GUEST_ADDRESS=$(
    aws ec2 describe-instances \
        --instance-ids "${INSTANCE_ID}" \
        --query 'Reservations[*].Instances[*].PublicIpAddress' \
        --output text
)
rm -f "$MARKET_OPTIONS" "$INSTANCE_OUT_INFO"

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS=$(wait_for_ssh_up "${PUBLIC_GUEST_ADDRESS}")
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check image installation result
check_result

greenprint "🕹 Get ostree install commit value"
INSTALL_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${PUBLIC_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${IGNITION_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes
ansible_become_method=sudo
ansible_become_pass=${IGNITION_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type="${OSTREE_OSNAME}" \
    -e ignition="true" \
    -e edge_type=edge-ami-image \
    -e ostree_commit="${INSTALL_HASH}" \
    -e sysroot_ro="$SYSROOT_RO" \
    -e fips="${FIPS}" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

##################################################################
##
## Upgrade and test edge EC2 instance with edge-ami image
##
##################################################################

# Write a blueprint for ostree image.
# NB: no ssh key in the upgrade commit because there is no home dir
tee "$BLUEPRINT_FILE" > /dev/null << EOF
name = "upgrade"
description = "An upgrade rhel-edge container image"
version = "0.0.2"
modules = []
groups = []

[[packages]]
name = "python3"
version = "*"

[[packages]]
name = "sssd"
version = "*"

[[packages]]
name = "wget"
version = "*"

[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "${EDGE_USER_PASSWORD_SHA512}"
home = "/home/admin/"
groups = ["wheel"]
EOF

# Red Hat does not provide realtime kernel package for ARM
if [[ "${ARCH}" != aarch64 ]]; then
    tee -a "$BLUEPRINT_FILE" > /dev/null << EOF
[customizations.kernel]
name = "kernel-rt"
EOF
fi

greenprint "📄 upgrade blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing upgrade blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve upgrade

# Build upgrade image.
build_image -b upgrade -t "${CONTAINER_TYPE}" -u "$PROD_REPO_URL"

# Download the image
greenprint "📥 Downloading the upgrade image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null

# Clear stage repo running env
greenprint "🧹 Clearing stage repo running env"
# Remove any status containers if exist
sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
# Remove all images
sudo podman rmi -f -a

# Deal with stage repo container
greenprint "🗜 Extracting image"
IMAGE_FILENAME="${COMPOSE_ID}-${CONTAINER_FILENAME}"
sudo podman pull "oci-archive:${IMAGE_FILENAME}"
sudo podman images
# Clear image file
sudo rm -f "$IMAGE_FILENAME"

# Run edge stage repo
greenprint "🛰 Running edge stage repo"
# Get image id to run image
EDGE_IMAGE_ID=$(sudo podman images --filter "dangling=true" --format "{{.ID}}")
sudo podman run -d --name rhel-edge --network edge --ip "$STAGE_REPO_ADDRESS" "$EDGE_IMAGE_ID"
# Wait for container to be running
until [ "$(sudo podman inspect -f '{{.State.Running}}' rhel-edge)" == "true" ]; do
    sleep 1;
done;

# Pull upgrade to prod mirror
greenprint "⛓ Pull upgrade to prod mirror"
sudo ostree --repo="$PROD_REPO" pull --mirror edge-stage "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO" static-delta generate "$OSTREE_REF"
sudo ostree --repo="$PROD_REPO" summary -u

# Get ostree commit value.
greenprint "🕹 Get ostree upgrade commit value"
UPGRADE_HASH=$(curl "${PROD_REPO_URL}/refs/heads/${OSTREE_REF}")

# Clean compose and blueprints.
greenprint "🧽 Clean up upgrade blueprint and compose"
sudo composer-cli compose delete "${COMPOSE_ID}" > /dev/null
sudo composer-cli blueprints delete upgrade > /dev/null

# Upload production repo to S3 Bucket
greenprint "Uploading upgraded production repo to AWS S3 Bucket"
# Avoid lock file issue permissions
sudo chmod 644 "${PROD_REPO}/.lock"
aws s3 cp \
    --quiet \
    --recursive \
    --acl public-read \
    "${PROD_REPO}/" \
    "${BUCKET_URL}/repo/"

# Replace edge-ami image remote repo URL
greenprint "Replacing default remote"
sudo ssh \
    "${SSH_OPTIONS[@]}" \
    -i "${SSH_KEY}" \
    admin@"${PUBLIC_GUEST_ADDRESS}" \
    "echo '${EDGE_USER_PASSWORD}' |sudo -S ostree remote delete rhel-edge"
sudo ssh \
    "${SSH_OPTIONS[@]}" \
    -i "${SSH_KEY}" \
    admin@"${PUBLIC_GUEST_ADDRESS}" \
    "echo '${EDGE_USER_PASSWORD}' |sudo -S ostree remote add --no-gpg-verify rhel-edge ${OBJECT_URL}/repo"

# Upgrade image/commit.
greenprint "🗳 Upgrade ostree image/commit"
sudo ssh \
    "${SSH_OPTIONS[@]}" \
    -i "${SSH_KEY}" \
    admin@"${PUBLIC_GUEST_ADDRESS}" \
    "echo '${EDGE_USER_PASSWORD}' |sudo -S rpm-ostree upgrade"
sudo ssh \
    "${SSH_OPTIONS[@]}" \
    -i "${SSH_KEY}" \
    admin@"${PUBLIC_GUEST_ADDRESS}" \
    "echo '${EDGE_USER_PASSWORD}' |nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure EC2 instance restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for _ in $(seq 0 30); do
    RESULTS=$(wait_for_ssh_up "${PUBLIC_GUEST_ADDRESS}")
    if [[ $RESULTS == 1 ]]; then
        echo "SSH is ready now! 🥳"
        break
    fi
    sleep 10
done

# Check ostree upgrade result
check_result

# Add instance IP address into /etc/ansible/hosts
sudo tee "${TEMPDIR}"/inventory > /dev/null << EOF
[ostree_guest]
${PUBLIC_GUEST_ADDRESS}

[ostree_guest:vars]
ansible_python_interpreter=/usr/bin/python3
ansible_user=${IGNITION_USER}
ansible_private_key_file=${SSH_KEY}
ansible_ssh_common_args="-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null"
ansible_become=yes
ansible_become_method=sudo
ansible_become_pass=${IGNITION_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type="${OSTREE_OSNAME}" \
    -e ignition="true" \
    -e edge_type=edge-ami-image \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e sysroot_ro="$SYSROOT_RO" \
    -e fips="${FIPS}" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Final success clean up
clean_up
aws_clean_up

exit 0
