#!/bin/bash
# AppSRE runs this script to build an ami and share it with an account
set -exv

COMMIT_SHA="${COMMIT_SHA:-$(git rev-parse HEAD)}"
COMMIT_BRANCH="${COMMIT_BRANCH:-$(git rev-parse --abbrev-ref HEAD)}"
SKIP_CREATE_AMI="${SKIP_CREATE_AMI:-false}"
BUILD_RPMS="${BUILD_RPMS:-true}"
# RHEL workers build their own rpms.
ANSIBLE_TAGS="${ANSIBLE_TAGS:-rhel}"
# Build rhel only
PACKER_ONLY_EXCEPT="${PACKER_ONLY_EXCEPT:---only=amazon-ebs.rhel-9-x86_64,amazon-ebs.rhel-9-aarch64}"

if [ -n "$GIT_BRANCH" ]; then
    # Use jenkins CI variables if available
    COMMIT_BRANCH="${GIT_BRANCH#*/}"
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

# prepare ansible inventories
function write_inventories {
    for item in templates/packer/ansible/inventory/*; do
        local distro_arch
        distro_arch="$(basename "$item")"

        # strip arch
        local distro="${distro_arch%-*}"

        # write rpmrepo_distribution variable
        local rpmrepo_distribution="$distro"
        if [[ $rpmrepo_distribution == rhel-9 ]]; then
            rpmrepo_distribution=rhel-9-cdn
        fi
        cat >"$item/group_vars/all.yml" <<EOF
---
rpmrepo_distribution: $rpmrepo_distribution
EOF

        # get distro name for schutzfile
        local schutzfile_distro="$distro"
        if [[ $schutzfile_distro == rhel-9 ]]; then
            schutzfile_distro=rhel-9.3
        fi

        # get osbuild_commit from schutzfile
        local osbuild_commit
        osbuild_commit=$(jq -r ".[\"$schutzfile_distro\"].dependencies.osbuild.commit" Schutzfile)

        # write osbuild_commit variable if defined in Schutzfile
        # if it's not defined, osbuild will be installed from distribution repositories
        if [[ $osbuild_commit != "null" ]]; then
            tee -a "$item/group_vars/all.yml" <<EOF
osbuild_commit: $osbuild_commit
EOF
        fi

    done
}

write_inventories

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
                   -e PACKER_IMAGE_USERS="$PACKER_IMAGE_USERS" \
                   -e PACKER_ONLY_EXCEPT="$PACKER_ONLY_EXCEPT" \
                   -e RH_ACTIVATION_KEY="$RH_ACTIVATION_KEY" \
                   -e RH_ORG_ID="$RH_ORG_ID" \
                   -e BUILD_RPMS="$BUILD_RPMS" \
                   -e PKR_VAR_aws_access_key="$PACKER_AWS_ACCESS_KEY_ID" \
                   -e PKR_VAR_aws_secret_key="$PACKER_AWS_SECRET_ACCESS_KEY" \
                   -e PKR_VAR_image_name="osbuild-composer-worker-$COMMIT_BRANCH-$COMMIT_SHA" \
                   -e PKR_VAR_composer_commit="$COMMIT_SHA" \
                   -e PKR_VAR_ansible_tags="$ANSIBLE_TAGS" \
                   -e PKR_VAR_skip_create_ami="$SKIP_CREATE_AMI" \
                   -e PKR_VAR_rh_activation_key="$RH_ACTIVATION_KEY" \
                   -e PKR_VAR_rh_org_id="$RH_ORG_ID" \
                   -e PYTHONUNBUFFERED=1 \
                   "packer:$COMMIT_SHA" /osbuild-composer/tools/appsre-worker-packer-container.sh
