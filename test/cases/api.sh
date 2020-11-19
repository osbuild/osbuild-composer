#!/usr/bin/bash

#
# Test osbuild-composer's main API endpoint by building a sample image and
# uploading it to AWS.
#
# This script sets `-x` and is meant to always be run like that. This is
# simpler than adding extensive error reporting, which would make this script
# considerably more complex. Also, the full trace this produces is very useful
# for the primary audience: developers of osbuild-composer looking at the log
# from a run on a remote continuous integration system.
#

set -euxo pipefail


#
# Provision the software under tet.
#

/usr/libexec/osbuild-composer-test/provision.sh


#
# Verify that this script is running in the right environment. In particular,
# it needs variables is set to access AWS.
#

printenv AWS_REGION AWS_BUCKET AWS_ACCESS_KEY_ID AWS_SECRET_ACCESS_KEY AWS_API_TEST_SHARE_ACCOUNT > /dev/null


#
# Create a temporary directory and ensure it gets deleted when this script
# terminates in any way.
#

WORKDIR=$(mktemp -d)
function cleanup() {
  rm -rf "$WORKDIR"
}
trap cleanup EXIT

#
# Install the aws client from the upstream release, because it doesn't seem to
# be available as a RHEL package.
#

if ! hash aws; then
    mkdir "$WORKDIR/aws"
    pushd "$WORKDIR/aws"
        curl -Ls --retry 5 --output awscliv2.zip \
            https://awscli.amazonaws.com/awscli-exe-linux-x86_64.zip
        unzip awscliv2.zip > /dev/null
        sudo ./aws/install > /dev/null
        aws --version
    popd
fi

AWS_CMD="aws --region $AWS_REGION --output json --color on"


#
# Prepare a request to be sent to the composer API.
#

REQUEST_FILE="${WORKDIR}/request.json"
ARCH=$(uname -m)
SNAPSHOT_NAME=$(uuidgen)

case $(set +x; . /etc/os-release; echo "$ID-$VERSION_ID") in
  "rhel-8.2" | "rhel-8.3")
    DISTRO="rhel-8"
  ;;
  "fedora-32")
    DISTRO="fedora-32"
  ;;
  "fedora-33")
    DISTRO="fedora-33"
  ;;
esac

cat > "$REQUEST_FILE" << EOF
{
  "distribution": "$DISTRO",
  "image_requests": [
    {
      "architecture": "$ARCH",
      "image_type": "ami",
      "repositories": $(jq ".\"$ARCH\"" /usr/share/tests/osbuild-composer/repositories/"$DISTRO".json),
      "upload_requests": [
        {
          "type": "aws",
          "options": {
            "region": "${AWS_REGION}",
            "s3": {
              "access_key_id": "${AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${AWS_SECRET_ACCESS_KEY}",
              "bucket": "${AWS_BUCKET}"
            },
            "ec2": {
              "access_key_id": "${AWS_ACCESS_KEY_ID}",
              "secret_access_key": "${AWS_SECRET_ACCESS_KEY}",
              "snapshot_name": "${SNAPSHOT_NAME}",
              "share_with_accounts": ["${AWS_API_TEST_SHARE_ACCOUNT}"]
            }
          }
        }
      ]
    }
  ]
}
EOF


#
# Send the request and wait for the job to finish.
#
# Separate `curl` and `jq` commands here, because piping them together hides
# the server's response in case of an error.
#

OUTPUT=$(curl \
  --silent \
  --show-error \
  --cacert /etc/osbuild-composer/ca-crt.pem \
  --key /etc/osbuild-composer/client-key.pem \
  --cert /etc/osbuild-composer/client-crt.pem \
  --header 'Content-Type: application/json' \
  --request POST \
  --data @"$REQUEST_FILE" \
  https://localhost/api/composer/v1/compose)

COMPOSE_ID=$(echo "$OUTPUT" | jq -r '.id')

while true
do
  OUTPUT=$(curl \
    --silent \
    --show-error \
    --cacert /etc/osbuild-composer/ca-crt.pem \
    --key /etc/osbuild-composer/client-key.pem \
    --cert /etc/osbuild-composer/client-crt.pem \
    https://localhost/api/composer/v1/compose/"$COMPOSE_ID")

  COMPOSE_STATUS=$(echo "$OUTPUT" | jq -r '.status')

  if [[ "$COMPOSE_STATUS" != "pending" && "$COMPOSE_STATUS" != "running" ]]; then
    test "$COMPOSE_STATUS" = "success"
    break
  fi

  sleep 30
done


#
# Verify the image landed in EC2, and delete it.
#

$AWS_CMD ec2 describe-images \
    --owners self \
    --filters Name=name,Values="$SNAPSHOT_NAME" \
    > "$WORKDIR/ami.json"

AMI_IMAGE_ID=$(jq -r '.Images[].ImageId' "$WORKDIR/ami.json")
SNAPSHOT_ID=$(jq -r '.Images[].BlockDeviceMappings[].Ebs.SnapshotId' "$WORKDIR/ami.json")
SHARE_OK=1

# Verify that the ec2 snapshot was shared
$AWS_CMD ec2 describe-snapshot-attribute --snapshot-id "$SNAPSHOT_ID" --attribute createVolumePermission > "$WORKDIR/snapshot-attributes.json"

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

$AWS_CMD ec2 deregister-image --image-id "$AMI_IMAGE_ID"
$AWS_CMD ec2 delete-snapshot --snapshot-id "$SNAPSHOT_ID"

if [ "$SHARE_OK" != 1 ]; then
    echo "EC2 snapshot wasn't shared with the AWS_API_TEST_SHARE_ACCOUNT. 😢"
    exit 1
fi

exit 0
