#!/bin/bash

set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

greenprint "📦 Installing epel"
sudo dnf install -y epel-release

greenprint "📦 Installing hashicorp repo"
sudo curl --location --output /etc/yum.repos.d/hashicorp.repo https://rpm.releases.hashicorp.com/RHEL/hashicorp.repo

greenprint "📦 Installing packages needed for this script"
sudo dnf install -y packer ansible jq

greenprint "🖼️ Building the image"

export PKR_VAR_aws_access_key="$PACKER_AWS_ACCESS_KEY_ID"
export PKR_VAR_aws_secret_key="$PACKER_AWS_SECRET_ACCESS_KEY"
export PKR_VAR_image_name="osbuild-composer-worker-$CI_COMMIT_BRANCH-$CI_COMMIT_SHA"
export PKR_VAR_composer_commit="$CI_COMMIT_SHA"

# use osbuild commit from Schutzfile
PKR_VAR_osbuild_commit=$(jq -r '.["rhel-8.4"].dependencies.osbuild.commit' Schutzfile)
export PKR_VAR_osbuild_commit

packer build templates/packer
