#!/bin/bash
# AppSRE runs this script to build an ami and share it with an account
set -exv


COMMIT_SHA=$(git rev-parse HEAD)
COMMIT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
ON_JENKINS=true
AMI_ID=ami-06f1e6f8b3457ae7c

# Use gitlab CI variables if available
if [ -n "$CI_COMMIT_SHA" ]; then
    ON_JENKINS=false
    COMMIT_SHA="$CI_COMMIT_SHA"
fi
if [ -n "$CI_COMMIT_BRANCH" ]; then
    COMMIT_BRANCH="$CI_COMMIT_BRANCH"
elif [ -n "$GIT_BRANCH" ]; then
    # Use jenkins CI variables if available
    COMMIT_BRANCH="${GIT_BRANCH#*/}"
fi

if [ "$ON_JENKINS" = false ]; then
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

function cleanup {
    set +e
    $CONTAINER_RUNTIME rmi "packer:$COMMIT_SHA"
}
trap cleanup EXIT

# What we will cp and exec
cat > worker-packer.sh<<'EOF'
#!/bin/bash
set -exv
EOF
chmod +x worker-packer.sh

function ec2_rpm_build {
    cat >> worker-packer.sh <<'EOF'
function cleanup {
    set +e
    if [ "$ON_JENKINS" = true ]; then
        if [ -n "$AWS_INSTANCE_ID" ]; then
            aws ec2 terminate-instances --instance-ids "$AWS_INSTANCE_ID"
        fi
        if [ -n "$KEY_NAME" ]; then
            aws ec2 delete-key-pair --key-name "$KEY_NAME"
        fi
    fi
}
trap cleanup EXIT

KEY_NAME=$(uuidgen)
RPMBUILD_DIR="/osbuild-composer/templates/packer/ansible/roles/common/files/rpmbuild/RPMS"
mkdir -p "$RPMBUILD_DIR"

aws ec2 create-key-pair --key-name "$KEY_NAME" --query 'KeyMaterial' --output text > /osbuild-composer/keypair.pem
chmod 600 /osbuild-composer/keypair.pem
aws ec2 run-instances --image-id "$PKR_VAR_ami_id" --instance-type c5.large --key-name "$KEY_NAME" \
    --tag-specifications "ResourceType=instance,Tags=[{Key=commit,Value=$COMMIT_SHA},{Key=name,Value=rpm-builder-$COMMIT_SHA}]" \
    > ./rpminstance.json
AWS_INSTANCE_ID=$(jq -r '.Instances[].InstanceId' "rpminstance.json")
aws ec2 wait instance-running --instance-ids "$AWS_INSTANCE_ID"

aws ec2 describe-instances --instance-ids "$AWS_INSTANCE_ID" > "instances.json"
RPMBUILDER_HOST=$(jq -r '.Reservations[].Instances[].PublicIpAddress' "instances.json")
for LOOP_COUNTER in {0..30}; do
    if ssh -i /osbuild-composer/keypair.pem -o ConnectTimeout=5 -o StrictHostKeyChecking=no "ec2-user@$RPMBUILDER_HOST" true; then
        break
    fi
    sleep 5
    echo "sleeping, try #$LOOP_COUNTER"
done

cat > /osbuild-composer/tools/appsre-ansible/inventory <<EOF2
[rpmbuilder]
$RPMBUILDER_HOST ansible_ssh_private_key_file=/osbuild-composer/keypair.pem ansible_ssh_common_args='-o StrictHostKeyChecking=no -o ServerAliveInterval=5'
EOF2

ansible-playbook \
    -i /osbuild-composer/tools/appsre-ansible/inventory \
    /osbuild-composer/tools/appsre-ansible/rpmbuild.yml \
    -e "COMPOSER_COMMIT=$COMMIT_SHA" \
    -e "OSBUILD_COMMIT=$(jq -r '.["rhel-8.5"].dependencies.osbuild.commit' /osbuild-composer/Schutzfile)" \
    -e "RH_ACTIVATION_KEY=$RH_ACTIVATION_KEY" \
    -e "RH_ORG_ID=$RH_ORG_ID"
EOF
}


# Use prebuilt rpms on CI
SKIP_TAGS="rpmcopy"
if [ "$ON_JENKINS" = true ]; then
    # Append rpm build to script when running on AppSRE's infra
    ec2_rpm_build
    SKIP_TAGS="rpmrepo"
fi

cat >> worker-packer.sh <<'EOF'
/usr/bin/packer build /osbuild-composer/templates/packer
EOF

greenprint "📦 Building the packer container"
$CONTAINER_RUNTIME build \
                   -f distribution/Dockerfile-ubi-packer \
                   -t "packer:$COMMIT_SHA" \
                   .

greenprint "🖼️ Building the image using packer container"
# Use an absolute path to packer binary to avoid conflicting cracklib-packer symling in /usr/sbin,
# installed during ansible installation process
$CONTAINER_RUNTIME run --rm \
                   -e AWS_ACCESS_KEY_ID="$PACKER_AWS_ACCESS_KEY_ID" \
                   -e AWS_SECRET_ACCESS_KEY="$PACKER_AWS_SECRET_ACCESS_KEY" \
                   -e AWS_DEFAULT_REGION="us-east-1" \
                   -e COMMIT_SHA="$COMMIT_SHA" \
                   -e ON_JENKINS="$ON_JENKINS" \
                   -e RH_ACTIVATION_KEY="$RH_ACTIVATION_KEY" \
                   -e RH_ORG_ID="$RH_ORG_ID" \
                   -e PKR_VAR_aws_access_key="$PACKER_AWS_ACCESS_KEY_ID" \
                   -e PKR_VAR_ami_id="$AMI_ID" \
                   -e PKR_VAR_aws_secret_key="$PACKER_AWS_SECRET_ACCESS_KEY" \
                   -e PKR_VAR_image_name="osbuild-composer-worker-$COMMIT_BRANCH-$COMMIT_SHA" \
                   -e PKR_VAR_composer_commit="$COMMIT_SHA" \
                   -e PKR_VAR_osbuild_commit="$(jq -r '.["rhel-8.4"].dependencies.osbuild.commit' Schutzfile)" \
                   -e PKR_VAR_ansible_skip_tags="$SKIP_TAGS" \
                   "packer:$COMMIT_SHA" /osbuild-composer/worker-packer.sh
