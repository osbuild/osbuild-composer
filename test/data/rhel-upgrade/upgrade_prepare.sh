#!/bin/bash

# This script upgrades the system to the latest target RHEL
set -xeuo pipefail

source /root/shared_lib.sh
source /etc/os-release

# Disable gpgcheck for internal repositories
echo "gpgcheck=0" >> /etc/yum.repos.d/baseos.repo
echo "gpgcheck=0" >> /etc/yum.repos.d/appstream.repo

# Install SUT
dnf install -y osbuild-composer composer-cli

# Expecting either 8 or 9
if [[ ${VERSION_ID%.*} == "8" ]]; then
  # Prepare the upgrade
  curl -k -o /etc/yum.repos.d/oam-group-leapp-rhel-8.repo https://gitlab.cee.redhat.com/oamg/upgrades-dev/oamg-rhel8-vagrant/-/raw/main/roles/init/files/leapp-copr.repo
  # install the leapp upgrade tool + other dependencies
  dnf install -y leapp-upgrade-el8toel9 vdo jq rpmdevtools
  # Get the COMPOSE_URL that we need
  source /root/define-compose-url.sh 9.8
elif [[ ${VERSION_ID%.*} == "9" ]]; then
  curl -k -o /etc/yum.repos.d/oam-group-leapp-rhel-9.repo https://gitlab.cee.redhat.com/oamg/upgrades-dev/oamg-rhel9-vagrant/-/raw/main/roles/init/files/leapp-copr.repo
  # install the leapp upgrade tool + other dependencies
  dnf install -y leapp-upgrade-el9toel10 vdo jq rpmdevtools
  # Get the COMPOSE_URL that we need
  source /root/define-compose-url.sh 10.2
else
  redprint "Running on unexpected VERSION_ID: ${VERSION_ID}"
  exit 1
fi

# prepare upgrade repositories
tee /etc/leapp/files/leapp_upgrade_repositories.repo > /dev/null << EOF
[APPSTREAM]
name=APPSTREAM
baseurl=${COMPOSE_URL}/compose/AppStream/x86_64/os/
enabled=0
gpgcheck=0

[BASEOS]
name=BASEOS
baseurl=${COMPOSE_URL}/compose/BaseOS/x86_64/os/
enabled=0
gpgcheck=0
EOF

# AllowZoneDrifting is disabled in RHEL-9, see rhbz#2054271 for more details
sed -i "s/^AllowZoneDrifting=.*/AllowZoneDrifting=no/" /etc/firewalld/firewalld.conf

# This user choice has to be made or else it inhibits the upgrade
leapp answer --add --section check_vdo.no_vdo_devices=True

export LEAPP_UNSUPPORTED=1
export LEAPP_DEVEL_DATABASE_SYNC_OFF=1

# upgrade
leapp upgrade --no-rhsm --reboot
