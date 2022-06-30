#!/usr/bin/bash

source /usr/libexec/tests/osbuild-composer/api/common/aws.sh
source /usr/libexec/tests/osbuild-composer/api/common/common.sh

function checkEnv() {
  printenv AWS_REGION AWS_BUCKET V2_AWS_ACCESS_KEY_ID V2_AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null
}

function cleanup() {
  # since this function can be called at any time, ensure that we don't expand unbound variables
  AWS_CMD="${AWS_CMD:-}"
  AWS_INSTANCE_ID="${AWS_INSTANCE_ID:-}"
  AMI_IMAGE_ID="${AMI_IMAGE_ID:-}"
  AWS_SNAPSHOT_ID="${AWS_SNAPSHOT_ID:-}"

  if [ -n "$AWS_CMD" ]; then
    $AWS_CMD ec2 terminate-instances --instance-ids "$AWS_INSTANCE_ID"
    $AWS_CMD ec2 deregister-image --image-id "$AMI_IMAGE_ID"
    $AWS_CMD ec2 delete-snapshot --snapshot-id "$AWS_SNAPSHOT_ID"
    $AWS_CMD ec2 delete-key-pair --key-name "key-for-$AMI_IMAGE_ID"
  fi
}


function createReqFile() {
  AWS_SNAPSHOT_NAME=${TEST_ID}

  cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "customizations": {
    "filesystem": [
      {
        "mountpoint": "/var",
        "min_size": 262144000
      }
    ],
    "payload_repositories": [
      {
        "baseurl": "$PAYLOAD_REPO_URL"
      }
    ],
    "packages": [
      "postgresql",
      "dummy"
    ]${SUBSCRIPTION_BLOCK},
    "users":[
      {
        "name": "user1",
        "groups": ["wheel"],
        "key": "$(cat "${WORKDIR}/usertest.pub")"
      },
      {
        "name": "user2",
        "key": "$(cat "${WORKDIR}/usertest.pub")"
      }
    ]
  },
  "image_request": {
      "architecture": "$ARCH",
      "image_type": "${IMAGE_TYPE}",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "upload_options": {
        "region": "${AWS_REGION}",
        "snapshot_name": "${AWS_SNAPSHOT_NAME}",
        "share_with_accounts": ["${AWS_API_TEST_SHARE_ACCOUNT}"]
    }
  }
}
EOF
}


function checkUploadStatusOptions() {
  local AMI
  AMI=$(echo "$UPLOAD_OPTIONS" | jq -r '.ami')
  local REGION
  REGION=$(echo "$UPLOAD_OPTIONS" | jq -r '.region')

  # AWS ID consist of resource identifier followed by a 17-character string
  echo "$AMI" | grep -e 'ami-[[:alnum:]]\{17\}' -
  test "$REGION" = "$AWS_REGION"
}

# Verify image in EC2 on AWS
function verify() {
  $AWS_CMD ec2 describe-images \
    --owners self \
    --filters Name=name,Values="$AWS_SNAPSHOT_NAME" \
    > "$WORKDIR/ami.json"

  AMI_IMAGE_ID=$(jq -r '.Images[].ImageId' "$WORKDIR/ami.json")
  AWS_SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' "$WORKDIR/ami.json")

  # Tag image and snapshot with "gitlab-ci-test" tag
  $AWS_CMD ec2 create-tags \
    --resources "${AWS_SNAPSHOT_ID}" "${AMI_IMAGE_ID}" \
    --tags Key=gitlab-ci-test,Value=true


  SHARE_OK=1

  # Verify that the ec2 snapshot was shared
  $AWS_CMD ec2 describe-snapshot-attribute --snapshot-id "$AWS_SNAPSHOT_ID" --attribute createVolumePermission > "$WORKDIR/snapshot-attributes.json"

  SHARED_ID=$(jq -r '.CreateVolumePermissions[0].UserId' "$WORKDIR/snapshot-attributes.json")
  if [ "$AWS_API_TEST_SHARE_ACCOUNT" != "$SHARED_ID" ]; then
    SHARE_OK=0
  fi

  # Verify that the ec2 ami was shared
  $AWS_CMD ec2 describe-image-attribute --image-id "$AMI_IMAGE_ID" --attribute launchPermission > "$WORKDIR/ami-attributes.json"

  SHARED_ID=$(jq -r '.LaunchPermissions[0].UserId' "$WORKDIR/ami-attributes.json")
  if [ "$AWS_API_TEST_SHARE_ACCOUNT" != "$SHARED_ID" ]; then
    SHARE_OK=0
  fi

  if [ "$SHARE_OK" != 1 ]; then
    echo "EC2 snapshot wasn't shared with the AWS_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
  fi

  # Create key-pair
  $AWS_CMD ec2 create-key-pair --key-name "key-for-$AMI_IMAGE_ID" --query 'KeyMaterial' --output text > keypair.pem
  chmod 400 ./keypair.pem

  # Create an instance based on the ami
  $AWS_CMD ec2 run-instances --image-id "$AMI_IMAGE_ID" --count 1 --instance-type t2.micro --key-name "key-for-$AMI_IMAGE_ID" --tag-specifications 'ResourceType=instance,Tags=[{Key=gitlab-ci-test,Value=true}]' > "$WORKDIR/instances.json"
  AWS_INSTANCE_ID=$(jq -r '.Instances[].InstanceId' "$WORKDIR/instances.json")

  $AWS_CMD ec2 wait instance-running --instance-ids "$AWS_INSTANCE_ID"

  $AWS_CMD ec2 describe-instances --instance-ids "$AWS_INSTANCE_ID" > "$WORKDIR/instances.json"
  HOST=$(jq -r '.Reservations[].Instances[].PublicIpAddress' "$WORKDIR/instances.json")

  echo "⏱ Waiting for AWS instance to respond to ssh"
  _instanceWaitSSH "$HOST"

  # Verify image
  _ssh="ssh -oStrictHostKeyChecking=no -i ./keypair.pem $SSH_USER@$HOST"
  _instanceCheck "$_ssh"

  # Check access to user1 and user2
  check_groups=$(ssh -oStrictHostKeyChecking=no -i "${WORKDIR}/usertest" "user1@$HOST" -t 'groups')
  if [[ $check_groups =~ "wheel" ]]; then
   echo "✔️  user1 has the group wheel"
  else
    echo 'user1 should have the group wheel 😢'
    exit 1
  fi
  check_groups=$(ssh -oStrictHostKeyChecking=no -i "${WORKDIR}/usertest" "user2@$HOST" -t 'groups')
  if [[ $check_groups =~ "wheel" ]]; then
    echo 'user2 should not have group wheel 😢'
    exit 1
  else
   echo "✔️  user2 does not have the group wheel"
  fi
}
