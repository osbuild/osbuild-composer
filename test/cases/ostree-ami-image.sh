#!/bin/bash
set -euo pipefail

# Provision the software under test.
/usr/libexec/osbuild-composer-test/provision.sh none

source /usr/libexec/tests/osbuild-composer/shared_lib.sh

# Get OS data.
source /etc/os-release
ARCH=$(uname -m)

# Start libvirtd and test it.
greenprint "🚀 Starting libvirt daemon"
sudo systemctl start libvirtd
sudo virsh list --all > /dev/null

# Install and start firewalld
greenprint "🔧 Install and start firewalld"
sudo dnf install -y firewalld
sudo systemctl enable --now firewalld

# Set a customized dnsmasq configuration for libvirt so we always get the
# same address on bootup.
sudo tee /tmp/integration.xml > /dev/null << EOF
<network>
  <name>integration</name>
  <uuid>1c8fe98c-b53a-4ca4-bbdb-deb0f26b3579</uuid>
  <forward mode='nat'>
    <nat>
      <port start='1024' end='65535'/>
    </nat>
  </forward>
  <bridge name='integration' zone='trusted' stp='on' delay='0'/>
  <mac address='52:54:00:36:46:ef'/>
  <ip address='192.168.100.1' netmask='255.255.255.0'>
    <dhcp>
      <range start='192.168.100.2' end='192.168.100.254'/>
      <host mac='34:49:22:B0:83:30' name='vm-bios' ip='192.168.100.50'/>
      <host mac='34:49:22:B0:83:31' name='vm-uefi' ip='192.168.100.51'/>
    </dhcp>
  </ip>
</network>
EOF

if ! sudo virsh net-info integration > /dev/null 2>&1; then
    sudo virsh net-define /tmp/integration.xml
fi  
if [[ $(sudo virsh net-info integration | grep 'Active' | awk '{print $2}') == 'no' ]]; then
    sudo virsh net-start integration
fi  

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
BUCKET_NAME="test-bucket-${TEST_UUID}"
BUCKET_URL="s3://${BUCKET_NAME}"
OBJECT_URL="http://${BUCKET_NAME}.s3.${AWS_DEFAULT_REGION}.amazonaws.com"

# Set up temporary files.
TEMPDIR=$(mktemp -d)
BLUEPRINT_FILE=${TEMPDIR}/blueprint.toml
COMPOSE_START=${TEMPDIR}/compose-start-${IMAGE_KEY}.json
COMPOSE_INFO=${TEMPDIR}/compose-info-${IMAGE_KEY}.json

# SSH setup.
SSH_OPTIONS=(-o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ConnectTimeout=5)
SSH_DATA_DIR=$(/usr/libexec/osbuild-composer-test/gen-ssh.sh)
SSH_KEY=${SSH_DATA_DIR}/id_rsa
SSH_KEY_PUB=$(cat "${SSH_KEY}".pub)
IGNITION_USER=core

case "${ID}-${VERSION_ID}" in
    "rhel-9.3")
        OSTREE_REF="rhel/9/${ARCH}/edge"
        SYSROOT_RO="true"
        ;;
    "centos-9")
        OSTREE_REF="centos/9/${ARCH}/edge"
        SYSROOT_RO="true"
        ;;
    *)
        echo "unsupported distro: ${ID}-${VERSION_ID}"
        exit 1;;
esac


# Get the compose log.
get_compose_log () {
    COMPOSE_ID=$1
    LOG_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.log

    # Download the logs.
    sudo composer-cli compose log "$COMPOSE_ID" | tee "$LOG_FILE" > /dev/null
}

# Get the compose metadata.
get_compose_metadata () {
    COMPOSE_ID=$1
    METADATA_FILE=${ARTIFACTS}/osbuild-${ID}-${VERSION_ID}-${COMPOSE_ID}.json

    # Download the metadata.
    sudo composer-cli compose metadata "$COMPOSE_ID" > /dev/null

    # Find the tarball and extract it.
    TARBALL=$(basename "$(find . -maxdepth 1 -type f -name "*-metadata.tar")")
    sudo tar -xf "$TARBALL" -C "${TEMPDIR}"
    sudo rm -f "$TARBALL"

    # Move the JSON file into place.
    sudo cat "${TEMPDIR}"/"${COMPOSE_ID}".json | jq -M '.' | tee "$METADATA_FILE" > /dev/null
}

# Build ostree image.
build_image() {
    blueprint_name=$1
    image_type=$2

    # Get worker unit file so we can watch the journal.
    WORKER_UNIT=$(sudo systemctl list-units | grep -o -E "osbuild.*worker.*\.service")
    sudo journalctl -af -n 1 -u "${WORKER_UNIT}" &
    WORKER_JOURNAL_PID=$!
    # Stop watching the worker journal when exiting.
    trap 'sudo pkill -P ${WORKER_JOURNAL_PID}' EXIT

    # Start the compose.
    greenprint "🚀 Starting compose"
    if [ $# -eq 3 ]; then
        repo_url=$3
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" --url "$repo_url" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    else
        sudo composer-cli --json compose start-ostree --ref "$OSTREE_REF" "$blueprint_name" "$image_type" | tee "$COMPOSE_START"
    fi
    COMPOSE_ID=$(get_build_info ".build_id" "$COMPOSE_START")

    # Wait for the compose to finish.
    greenprint "⏱ Waiting for compose to finish: ${COMPOSE_ID}"
    while true; do
        sudo composer-cli --json compose info "${COMPOSE_ID}" | tee "$COMPOSE_INFO" > /dev/null
        COMPOSE_STATUS=$(get_build_info ".queue_status" "$COMPOSE_INFO")

        # Is the compose finished?
        if [[ $COMPOSE_STATUS != RUNNING ]] && [[ $COMPOSE_STATUS != WAITING ]]; then
            break
        fi

        # Wait 30 seconds and try again.
        sleep 5
    done

    # Capture the compose logs from osbuild.
    greenprint "💬 Getting compose log and metadata"
    get_compose_log "$COMPOSE_ID"
    get_compose_metadata "$COMPOSE_ID"

    # Kill the journal monitor immediately and remove the trap
    sudo pkill -P ${WORKER_JOURNAL_PID}
    trap - EXIT

    # Did the compose finish with success?
    if [[ $COMPOSE_STATUS != FINISHED ]]; then
        echo "Something went wrong with the compose. 😢"
        exit 1
    fi
}

# Wait for the ssh server up to be.
wait_for_ssh_up () {
    SSH_STATUS=$(sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@"${1}" '/bin/bash -c "echo -n READY"')
    if [[ $SSH_STATUS == READY ]]; then
        echo 1
    else
        echo 0
    fi
}

# Clean up our mess.
clean_up () {
    greenprint "🧼 Cleaning up"

    # Remove any status containers if exist
    sudo podman ps -a -q --format "{{.ID}}" | sudo xargs --no-run-if-empty podman rm -f
    # Remove all images
    sudo podman rmi -f -a

    # Remove prod repo
    sudo rm -rf "$PROD_REPO"
    
    # Remomve tmp dir.
    sudo rm -rf "$TEMPDIR"

    # Stop prod repo http service
    sudo systemctl disable --now httpd

    # Deregister edge AMI image
    aws ec2 deregister-image --image-id "${AMI_ID}"
    
    # Remove snapshot
    aws ec2 delete-snapshot --snapshot-id "${SNAPSHOT_ID}"
    
    # Delete Key Pair
    aws ec2 delete-key-pair --key-name "${AMI_KEY_NAME}"
    
    # Terminate running instance
    aws ec2 terminate-instances --instance-ids "${INSTANCE_ID}"
    
    # Clean up local folder
    sudo rm -rf "${CONTAINERS_FILE}" "${IMPORT_SNAPSHOT_INFO}" "${IMPORT_SNAPSHOT_TASK}" "${AMI_FILENAME}" "${REGISTERED_AMI_ID}" "${INSTANCE_OUT_INFO}" "${MARKET_OPTIONS}" "${IGW_OUTPUT}" "${RT_OUTPUT}" "${SG_OUTPUT}" "${SUBNET_OUTPUT}" "${VPC_OUTPUT}"
    
    # Remove bucket content and bucket itself quietly
    aws s3 rb "${BUCKET_URL}" --force > /dev/null
    
    # Remove subnet
    aws ec2 delete-subnet --subnet-id "${SUBN_ID}"
    
    # Remove Security Groups
    aws ec2 delete-security-group --group-id "${SEC_GROUP_ID}"
    
    # Remove Route Table
    aws ec2 delete-route-table --route-table-id "${RT_ID}"
    
    # Detach Internet gateway from VPC
    aws ec2 detach-internet-gateway --internet-gateway-id "${IGW_ID}" --vpc-id "${VPC_ID}"
    
    # Remove Internet gateway
    aws ec2 delete-internet-gateway --internet-gateway-id "${IGW_ID}"
    
    # Delete VPC
    aws ec2 delete-vpc --vpc-id "${VPC_ID}"

}

# Test result checking
check_result () {
    greenprint "🎏 Checking for test result"
    if [[ $RESULTS == 1 ]]; then
        greenprint "💚 Success"
    else
        greenprint "❌ Failed"
        clean_up
        exit 1
    fi
}

# AWS EC2 AMI tagging function
tag_ec2_ami () {
    ami_id=$1
    
    greenprint "Add custom tags to EC2 ami"
    aws ec2 create-tags \
      --resources "${ami_id}" --tags Key=Project,Value=rhel-edge
    aws ec2 create-tags \
      --resources "${ami_id}" --tags Key=ImageType,Value=edge-ami
    aws ec2 create-tags \
      --resources "${ami_id}" --tags Key=BuildBy,Value=osbuild-composer

}

# AWS EC2 instance tagging function
tag_ec2_instance () {
    instance_id=$1

    aws ec2 create-tags \
      --resources "${instance_id}" --tags Key=Project,Value=rhel-edge
}

tag_describe_resource () {
    res_id=$1
    aws ec2 describe-tags \
    --filters "Name=resource-id,Values=${res_id}"
}

# Get instance type
get_instance_type () {
    arch=$1
    
    if [[ $arch == x86_64 ]]; then
        allInstanceTypes=("t2.medium" \
            "t3.medium" \
            "m6a.large")
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

[customizations.kernel]
name = "kernel-rt"
EOF

greenprint "📄 container blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing container blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve container

# Build container image.
build_image container "${CONTAINER_TYPE}"

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
        "passwordHash": "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl.",
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
curl "https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip" -o "awscliv2.zip"
unzip awscliv2.zip > /dev/null
sudo ./aws/install --update
aws --version

# Clean up unzipped folder and files
sudo rm -rf awscliv2.zip ./aws

# Create Bucket
aws s3 mb "${BUCKET_URL}" --region "${AWS_DEFAULT_REGION}"
# Disable Public Access Block
aws s3api put-public-access-block --bucket "${BUCKET_NAME}" --public-access-block-configuration "BlockPublicAcls=false,IgnorePublicAcls=false,BlockPublicPolicy=false,RestrictPublicBuckets=false"
# Set Object ownership
aws s3api put-bucket-ownership-controls --bucket "${BUCKET_NAME}" --ownership-controls="Rules=[{ObjectOwnership=BucketOwnerPreferred}]"

# Upload ignition files to bucket
greenprint "📂 Upload ignition files to AWS S3 bucket"
aws s3 cp "${IGNITION_CONFIG_PATH}" "${BUCKET_URL}/" --acl public-read
aws s3 cp "${IGNITION_CONFIG_SAMPLE_PATH}" "${BUCKET_URL}/" --acl public-read
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

[[customizations.user]]
name = "admin"
description = "Administrator account"
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
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
build_image ami "${AMI_IMAGE_TYPE}" "${PROD_REPO_URL}"

# Download the image
greenprint "📥 Downloading the ami image"
sudo composer-cli compose image "${COMPOSE_ID}" > /dev/null
AMI_FILENAME="${COMPOSE_ID}-${AMI_IMAGE_FILENAME}"
# Configure ami file with correct permissions
sudo chmod +r "${AMI_FILENAME}"

# Upload ami to AWS S3 bucket
greenprint "📂 Upload raw ami to S3 bucket"
aws s3 cp --quiet "${AMI_FILENAME}" "${BUCKET_URL}/" --acl public-read
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
IMPORT_SNAPSHOT_INFO=output_snapshot_info.json
aws ec2 import-snapshot --description "RHEL edge ami snapshot" --disk-container file://"${CONTAINERS_FILE}" > "${IMPORT_SNAPSHOT_INFO}"
IMPORT_TASK_ID=$(cat "${IMPORT_SNAPSHOT_INFO}" | jq -r '.ImportTaskId')

# Monitor snapshot status
greenprint "Check import status of the snapshot"
IMPORT_SNAPSHOT_TASK=output_snapshot_task.json
while true; do
    aws ec2 describe-import-snapshot-tasks --import-task-ids "${IMPORT_TASK_ID}" | tee "${IMPORT_SNAPSHOT_TASK}" > /dev/null
    IMPORT_STATUS=$(cat "${IMPORT_SNAPSHOT_TASK}" | jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.Status')

    # Has the snapshot finished?
    if [[ $IMPORT_STATUS != active ]]; then
        break
    fi
    
    # Wait 5 seconds and try again.
    sleep 5
done

if [[ $IMPORT_STATUS != completed ]]; then
  echo "Something went wrong with the snapshot. 😢"
  exit 1
else
  greenprint "Snapshot imported successfully."
fi

# Import  keypair
greenprint "Share ssh public key with AWS"
AMI_KEY_NAME="edge-ami-key-${TEST_UUID}"
# Clean previous configured keypair
aws ec2 delete-key-pair --key-name "${AMI_KEY_NAME}"
aws ec2 import-key-pair --key-name "${AMI_KEY_NAME}" --public-key-material fileb://"${SSH_KEY}".pub

# Network setup
greenprint "VPC Network setup."

# Create VPC
VPC_OUTPUT=vpc_output.json
aws ec2 create-vpc --cidr-block 172.32.0.0/16 --region="${AWS_DEFAULT_REGION}" | tee "${VPC_OUTPUT}" > /dev/null
VPC_ID=$(cat "${VPC_OUTPUT}" | jq -r '.Vpc.VpcId')

# Create VPC Internet Gateway
IGW_OUTPUT=igw_output.json
aws ec2 create-internet-gateway | tee "${IGW_OUTPUT}" > /dev/null
IGW_ID=$(cat "${IGW_OUTPUT}" | jq -r '.InternetGateway.InternetGatewayId')

# Attach internet gateway 
aws ec2 attach-internet-gateway --vpc-id "${VPC_ID}" --internet-gateway-id "${IGW_ID}"

# Create Subnet for VPC
SUBNET_OUTPUT=sub_net_output.json
aws ec2 create-subnet --vpc-id "${VPC_ID}" --cidr-block 172.32.32.0/24 | tee "${SUBNET_OUTPUT}"
SUBN_ID=$(cat "${SUBNET_OUTPUT}" | jq -r '.Subnet.SubnetId')

# Add default route in route table for all vpc subnets
# Create route table
RT_OUTPUT=route_table_out.json
aws ec2 create-route-table --vpc-id "${VPC_ID}" | tee "${RT_OUTPUT}" > /dev/null
RT_ID=$(cat "${RT_OUTPUT}" | jq -r '.RouteTable.RouteTableId')
aws ec2 create-route --route-table-id "${RT_ID}" --destination-cidr-block 0.0.0.0/0 --gateway-id "${IGW_ID}"
# Associate route table to subnet
aws ec2 associate-route-table --route-table-id "${RT_ID}" --subnet-id "${SUBN_ID}"

# Security Group setup
SG_OUTPUT=sec_group.json
aws ec2 create-security-group --group-name mysecuritygroup --description "edge-ami security group" --vpc-id "${VPC_ID}" | tee "${SG_OUTPUT}"
SEC_GROUP_ID=$(cat "${SG_OUTPUT}" | jq -r '.GroupId')
# Allow inbound ssh connections
aws ec2 authorize-security-group-ingress --group-id "${SEC_GROUP_ID}" --protocol tcp --port 22 --cidr 0.0.0.0/0

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

##################################################################
##
## Install and test edge EC2 instance with edge-ami image
##
##################################################################
# Create AMI image from EBS snapshot
greenprint "Register AMI, create image from snapshot."
REGISTERED_AMI_NAME="edge_ami-${TEST_UUID}"
REGISTERED_AMI_ID=output_ami_id.json
SNAPSHOT_ID=$(cat "${IMPORT_SNAPSHOT_TASK}" | jq -r '.ImportSnapshotTasks[].SnapshotTaskDetail.SnapshotId')
aws ec2 register-image \
    --name "${REGISTERED_AMI_NAME}" \
    --root-device-name /dev/xvda \
    --architecture "${ARCH}" \
    --ena-support \
    --sriov-net-support simple \
    --virtualization-type hvm \
    --block-device-mappings DeviceName=/dev/xvda,Ebs=\{SnapshotId="${SNAPSHOT_ID}"\} DeviceName=/dev/xvdf,Ebs=\{VolumeSize=10\} \
    --boot-mode uefi-preferred \
    --output json > "${REGISTERED_AMI_ID}"

AMI_ID=$(cat "${REGISTERED_AMI_ID}" | jq -r '.ImageId')
tag_ec2_ami "${AMI_ID}"
tag_describe_resource "${AMI_ID}"

# Launch Instance
greenprint "💻 Launch instance from AMI"
for LOOP_COUNTER in $(seq 0 9); do
    INSTANCE_OUT_INFO=instance_output_info.json
    INSTANCE_TYPE=$(get_instance_type "${ARCH}")
    aws ec2 run-instances \
        --image-id "${AMI_ID}" \
        --count 1 \
        --instance-type "${INSTANCE_TYPE}" \
        --instance-market-options file://"${MARKET_OPTIONS}" \
        --key-name "${AMI_KEY_NAME}" \
        --security-group-ids "${SEC_GROUP_ID}" \
        --subnet-id "${SUBN_ID}" \
        --associate-public-ip-address > "${INSTANCE_OUT_INFO}" 2>&1 || :
    if ! grep -iqE 'unsupported|InsufficientInstanceCapacity' "${INSTANCE_OUT_INFO}"; then
        echo "Instance type supported!"
        break
    fi
    sleep 30
done
cat "${INSTANCE_OUT_INFO}"

# wait for instance running
sleep 5

# get instance public ip
INSTANCE_ID=$(jq -r '.Instances[].InstanceId' "${INSTANCE_OUT_INFO}")
tag_ec2_instance "${INSTANCE_ID}"
tag_describe_resource "${INSTANCE_ID}"
PUBLIC_GUEST_ADDRESS=$(aws ec2 describe-instances --instance-ids "${INSTANCE_ID}" --query 'Reservations[*].Instances[*].PublicIpAddress' --output text)

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
for LOOP_COUNTER in $(seq 0 30); do
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
ansible_become_pass=${EDGE_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type="${OSTREE_OSNAME}" \
    -e skip_rollback_test="true" \
    -e ignition="true" \
    -e edge_type=edge-ami-image \
    -e ostree_commit="${INSTALL_HASH}" \
    -e sysroot_ro="$SYSROOT_RO" \
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
password = "\$6\$GRmb7S0p8vsYmXzH\$o0E020S.9JQGaHkszoog4ha4AQVs3sk8q0DvLjSMxoxHBKnB2FBXGQ/OkwZQfW/76ktHd0NX5nls2LPxPuUdl."
home = "/home/admin/"
groups = ["wheel"]

[customizations.kernel]
name = "kernel-rt"
EOF

greenprint "📄 upgrade blueprint"
cat "$BLUEPRINT_FILE"

# Prepare the blueprint for the compose.
greenprint "📋 Preparing upgrade blueprint"
sudo composer-cli blueprints push "$BLUEPRINT_FILE"
sudo composer-cli blueprints depsolve upgrade

# Build upgrade image.
build_image upgrade  "${CONTAINER_TYPE}" "$PROD_REPO_URL"

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
aws s3 cp --quiet --recursive --acl public-read "${PROD_REPO}/" "${BUCKET_URL}/repo/"

# Replace edge-ami image remote repo URL
greenprint "Replacing default remote"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@"${PUBLIC_GUEST_ADDRESS}" "echo ${EDGE_USER_PASSWORD} |sudo -S ostree remote delete rhel-edge"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@"${PUBLIC_GUEST_ADDRESS}" "echo ${EDGE_USER_PASSWORD} |sudo -S ostree remote add --no-gpg-verify rhel-edge ${OBJECT_URL}/repo"

# Upgrade image/commit.
greenprint "🗳 Upgrade ostree image/commit"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@"${PUBLIC_GUEST_ADDRESS}" "echo ${EDGE_USER_PASSWORD} |sudo -S rpm-ostree upgrade"
sudo ssh "${SSH_OPTIONS[@]}" -i "${SSH_KEY}" admin@"${PUBLIC_GUEST_ADDRESS}" "echo ${EDGE_USER_PASSWORD} |nohup sudo -S systemctl reboot &>/dev/null & exit"

# Sleep 10 seconds here to make sure EC2 instance restarted already
sleep 10

# Check for ssh ready to go.
greenprint "🛃 Checking for SSH is ready to go"
# shellcheck disable=SC2034  # Unused variables left for readability
for LOOP_COUNTER in $(seq 0 30); do
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
ansible_become_pass=${EDGE_USER_PASSWORD}
EOF

# Test IoT/Edge OS
sudo ansible-playbook -v -i "${TEMPDIR}"/inventory \
    -e image_type="${OSTREE_OSNAME}" \
    -e ignition="true" \
    -e edge_type=edge-ami-image \
    -e ostree_commit="${UPGRADE_HASH}" \
    -e sysroot_ro="$SYSROOT_RO" \
    /usr/share/tests/osbuild-composer/ansible/check_ostree.yaml || RESULTS=0
check_result

# Final success clean up
clean_up

exit 0

