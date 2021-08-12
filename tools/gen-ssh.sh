#!/usr/bin/bash

# Create SSH key
SSH_DATA_DIR="$(mktemp -d)"
SSH_KEY=${SSH_DATA_DIR}/id_rsa
ssh-keygen -f "${SSH_KEY}" -N "" -q -t rsa

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
