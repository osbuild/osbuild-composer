#!/usr/bin/bash

# Create SSH key
SSH_DATA_DIR="$(mktemp -d)"
SSH_KEY=${SSH_DATA_DIR}/id_rsa

# openssl gets installed as a dependency of the osbuild-composer-tests but it
# might not update openssh at the same time, which can cause a version mismatch
# when running ssh-keygen:
#
#   OpenSSL version mismatch. Built against 30000000, you have 30200010
#
# Make sure openssh is up to date before running ssh-keygen
sudo dnf -y upgrade openssh > /dev/null
ssh-keygen -f "${SSH_KEY}" -N "" -q -t rsa-sha2-256 -b 2048

# Change cloud-init/user-data ssh key
key=" - $(cat "${SSH_KEY}".pub)"
# Temporary, will copy user data from cloud-init once
# go test are updated
tee "${SSH_DATA_DIR}"/user-data > /dev/null << EOF
#cloud-config
write_files:
  - path: "/etc/smoke-test.txt"
    content: "c21va2UtdGVzdAo="
    encoding: "b64"
    owner: "root:root"
    permissions: "0644"

user: redhat
ssh_authorized_keys:
${key}
EOF

# Return temp directory
echo "${SSH_DATA_DIR}"
