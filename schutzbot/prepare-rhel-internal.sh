#!/bin/bash
set -euo pipefail

# Colorful output.
function greenprint {
  echo -e "\033[1;32m[$(date -Isecond)] ${1}\033[0m"
}

ALL_ARCHES="aarch64 ppc64le s390x x86_64"

if [ -e ../tools/define-compose-url.sh ]
then
    source ../tools/define-compose-url.sh
else
    source ./tools/define-compose-url.sh
fi

# Create a repository file for installing the osbuild-composer RPMs
greenprint "📜 Generating dnf repository file"
rm -f rhel"${VERSION_ID%.*}"internal.repo
for ARCH in $ALL_ARCHES; do
    tee -a rhel"${VERSION_ID%.*}"internal.repo << EOF

[rhel${VERSION_ID}-internal-baseos-${ARCH}]
name=RHEL Internal BaseOS
baseurl=${COMPOSE_URL}/compose/BaseOS/${ARCH}/os/
enabled=1
gpgcheck=0
# Default dnf repo priority is 99. Lower number means higher priority.
priority=1

[rhel${VERSION_ID}-internal-appstream-${ARCH}]
name=RHEL Internal AppStream
baseurl=${COMPOSE_URL}/compose/AppStream/${ARCH}/os/
enabled=1
gpgcheck=0
# osbuild-composer repo priority is 5
priority=1
EOF
done

# Create tests .repo file if REPO_URL is provided from ENV
# Otherwise osbuild-composer-tests.rpm will be downloaded from
# existing repositories
if [ -n "${REPO_URL+x}" ]; then
    JOB_NAME="${JOB_NAME:-${CI_JOB_ID}}"

    greenprint "📜 Amend dnf repository file"
    tee -a rhel"${VERSION_ID%.*}"internal.repo << EOF

[osbuild-composer-tests-multi-arch]
name=Tests ${JOB_NAME}
baseurl=${REPO_URL}
enabled=1
gpgcheck=0
# osbuild-composer repo priority is 5
priority=1
EOF

fi
