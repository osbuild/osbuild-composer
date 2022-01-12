#!/bin/bash
# AppSRE runs this script to build an ami and share it with an account
set -exv


COMMIT_SHA=$(git rev-parse HEAD)
COMMIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)

# Use CI variables if available
if [ -n "$CI_COMMIT_SHA" ]; then
    COMMIT_SHA="$CI_COMMIT_SHA"
fi
if [ -n "$CI_COMMIT_BRANCH" ]; then
    COMMIT_BRANCH="$CI_COMMIT_BRANCH"
fi

# $WORKSPACE is set by jenkins and in gitlab,
# for gitlab change it to the current directory
if [ -n "$CI_COMMIT_SHA" ]; then
    WORKSPACE="$PWD"
fi

if [ -n "$CI_COMMIT_SHA" ]; then
    sudo dnf install -y podman jq
fi

# decide whether podman or docker should be used
if which podman 2>/dev/null >&2; then
    CONTAINER_RUNTIME=podman
elif which docker 2>/dev/null >&2; then
    CONTAINER_RUNTIME="docker --config=$PWD/.docker"
else
    echo No container runtime found, install podman or docker.
    exit 2
fi

function greenprint {
    echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

KEY_NAME=$(uuidgen)
function cleanup {
    set +e
    if [ -z "$CI_COMMIT_SHA" ]; then
        if [ -n "$AWS_INSTANCE_ID" ]; then
            $CONTAINER_RUNTIME run --rm \
                               -e AWS_ACCESS_KEY_ID="$PACKER_AWS_ACCESS_KEY_ID" \
                               -e AWS_SECRET_ACCESS_KEY="$PACKER_AWS_SECRET_ACCESS_KEY" \
                               -e AWS_DEFAULT_REGION="us-east-1" \
                               "packer:$COMMIT_SHA" aws ec2 terminate-instances \
                               --instance-ids "$AWS_INSTANCE_ID"
        fi
        $CONTAINER_RUNTIME run --rm \
                           -e AWS_ACCESS_KEY_ID="$PACKER_AWS_ACCESS_KEY_ID" \
                           -e AWS_SECRET_ACCESS_KEY="$PACKER_AWS_SECRET_ACCESS_KEY" \
                           -e AWS_DEFAULT_REGION="us-east-1" \
                           "packer:$COMMIT_SHA" aws ec2 delete-key-pair --key-name "$KEY_NAME"
    fi

    $CONTAINER_RUNTIME rmi "packer:$COMMIT_SHA"
}
trap cleanup EXIT

function ec2_rpm_build {
    RPMBUILD_DIR="./templates/packer/ansible/roles/common/files/rpmbuild/RPMS"
    mkdir -p "$RPMBUILD_DIR"

    greenprint "🚀 Start RHEL Cloud Access image to build rpms on"
    $CONTAINER_RUNTIME run --rm \
                       -e AWS_ACCESS_KEY_ID="$PACKER_AWS_ACCESS_KEY_ID" \
                       -e AWS_SECRET_ACCESS_KEY="$PACKER_AWS_SECRET_ACCESS_KEY" \
                       -e AWS_DEFAULT_REGION="us-east-1" \
                       "packer:$COMMIT_SHA" aws ec2 create-key-pair \
                       --key-name "$KEY_NAME" \
                       --query 'KeyMaterial' \
                       --output text \
                       > ./keypair.pem
    chmod 600 ./keypair.pem

    $CONTAINER_RUNTIME run --rm \
                       -e AWS_ACCESS_KEY_ID="$PACKER_AWS_ACCESS_KEY_ID" \
                       -e AWS_SECRET_ACCESS_KEY="$PACKER_AWS_SECRET_ACCESS_KEY" \
                       -e AWS_DEFAULT_REGION="us-east-1" \
                       "packer:$COMMIT_SHA" aws ec2 run-instances \
                       --image-id ami-0b0af3577fe5e3532 --instance-type c5.large \
                       --key-name "$KEY_NAME" \
                       --tag-specifications "ResourceType=instance,Tags=[{Key=commit,Value=$COMMIT_SHA},{Key=name,Value=rpm-builder-$COMMIT_SHA}]" \
                       > ./rpminstance.json
    AWS_INSTANCE_ID=$(jq -r '.Instances[].InstanceId' "rpminstance.json")

    $CONTAINER_RUNTIME run --rm \
                       -e AWS_ACCESS_KEY_ID="$PACKER_AWS_ACCESS_KEY_ID" \
                       -e AWS_SECRET_ACCESS_KEY="$PACKER_AWS_SECRET_ACCESS_KEY" \
                       -e AWS_DEFAULT_REGION="us-east-1" \
                       "packer:$COMMIT_SHA" aws ec2 wait instance-running \
                       --instance-ids "$AWS_INSTANCE_ID"

    $CONTAINER_RUNTIME run --rm \
                       -e AWS_ACCESS_KEY_ID="$PACKER_AWS_ACCESS_KEY_ID" \
                       -e AWS_SECRET_ACCESS_KEY="$PACKER_AWS_SECRET_ACCESS_KEY" \
                       -e AWS_DEFAULT_REGION="us-east-1" \
                       "packer:$COMMIT_SHA" aws ec2 describe-instances \
                       --instance-ids "$AWS_INSTANCE_ID" \
                       > "instances.json"
    RPMBUILDER_HOST=$(jq -r '.Reservations[].Instances[].PublicIpAddress' "instances.json")


    for LOOP_COUNTER in {0..30}; do
        if ssh -i ./keypair.pem -o ConnectTimeout=10 -o StrictHostKeyChecking=no "ec2-user@$RPMBUILDER_HOST" true > /dev/null 2>&1; then
            break
        fi
        echo "sleeping, try #$LOOP_COUNTER"
    done

    cat > tools/appsre-ansible/inventory <<EOF
[rpmbuilder]
$RPMBUILDER_HOST ansible_ssh_private_key_file=/osbuild-composer/keypair.pem ansible_ssh_common_args='-o StrictHostKeyChecking=no'
EOF

    greenprint "📦 Building the rpms"
    $CONTAINER_RUNTIME run --rm \
                       -v "$WORKSPACE:/osbuild-composer:z" \
                       "packer:$COMMIT_SHA" ansible-playbook \
                       -i /osbuild-composer/tools/appsre-ansible/inventory \
                       /osbuild-composer/tools/appsre-ansible/rpmbuild.yml \
                       -e "COMPOSER_COMMIT=$COMMIT_SHA" \
                       -e "OSBUILD_COMMIT=$(jq -r '.["rhel-8.4"].dependencies.osbuild.commit' Schutzfile)"
}

greenprint "📦 Building the packer container"
$CONTAINER_RUNTIME build \
                   -f distribution/Dockerfile-ubi-packer \
                   -t "packer:$COMMIT_SHA" \
                   .

if [ -n "$CI_COMMIT_SHA" ]; then
    # Use prebuilt rpms on CI
    SKIP_TAGS="rpmcopy"
else
    # Build rpms when running on AppSRE's infra
    ec2_rpm_build
    SKIP_TAGS="rpmrepo"
fi

# Format: PACKER_IMAGE_USERS="\"000000000000\",\"000000000001\""
if [ -n "$PACKER_IMAGE_USERS" ]; then
    cat > templates/packer/share.auto.pkrvars.hcl <<EOF
image_users = [$PACKER_IMAGE_USERS]
EOF
fi

greenprint "🖼️ Building the image using packer container"
# Use an absolute path to packer binary to avoid conflicting cracklib-packer symling in /usr/sbin,
# installed during ansible installation process
$CONTAINER_RUNTIME run --rm \
                   -e PKR_VAR_aws_access_key="$PACKER_AWS_ACCESS_KEY_ID" \
                   -e PKR_VAR_aws_secret_key="$PACKER_AWS_SECRET_ACCESS_KEY" \
                   -e PKR_VAR_image_name="osbuild-composer-worker-$COMMIT_BRANCH-$COMMIT_SHA" \
                   -e PKR_VAR_composer_commit="$COMMIT_SHA" \
                   -e PKR_VAR_osbuild_commit="$(jq -r '.["rhel-8.4"].dependencies.osbuild.commit' Schutzfile)" \
                   -e PKR_VAR_ansible_skip_tags="$SKIP_TAGS" \
                   -v "$WORKSPACE:/osbuild-composer:z" \
                   "packer:$COMMIT_SHA" /usr/bin/packer build /osbuild-composer/templates/packer
