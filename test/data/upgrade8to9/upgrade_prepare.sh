#!/bin/bash

# This upgrades the system to the latest RHEL-9.x
set -xeuo pipefail

# Disable gpgcheck for internal repositories
echo "gpgcheck=0" >> /etc/yum.repos.d/baseos.repo
echo "gpgcheck=0" >> /etc/yum.repos.d/appstream.repo

# Install SUT
dnf install -y osbuild-composer composer-cli

# Prepare the upgrade
curl -k -o /etc/yum.repos.d/oam-group-leapp-rhel-8.repo https://gitlab.cee.redhat.com/oamg/upgrades-dev/oamg-rhel8-vagrant/-/raw/main/roles/init/files/leapp-copr.repo
# install the leapp upgrade tool + other dependencies
dnf install -y leapp-upgrade-el8toel9 vdo jq rpmdevtools

# prepare upgrade repositories
tee /etc/leapp/files/leapp_upgrade_repositories.repo > /dev/null << EOF
[APPSTREAM]
name=APPSTREAM
baseurl=http://download.devel.redhat.com/rhel-9/nightly/RHEL-9/latest-RHEL-9.5.0/compose/AppStream/x86_64/os/
enabled=0
gpgcheck=0

[BASEOS]
name=BASEOS
baseurl=http://download.devel.redhat.com/rhel-9/nightly/RHEL-9/latest-RHEL-9.5.0/compose/BaseOS/x86_64/os/
enabled=0
gpgcheck=0
EOF

# AllowZoneDrifting is disabled in RHEL-9, see rhbz#2054271 for more details
sed -i "s/^AllowZoneDrifting=.*/AllowZoneDrifting=no/" /etc/firewalld/firewalld.conf

# This user choice has to be made or else it inhibits the upgrade
leapp answer --add --section check_vdo.no_vdo_devices=True

# check upgrade
leapp preupgrade --debug --no-rhsm

export LEAPP_UNSUPPORTED=1
export LEAPP_DEVEL_DATABASE_SYNC_OFF=1

# upgrade
leapp upgrade --debug --no-rhsm --reboot
